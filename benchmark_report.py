#!/usr/bin/env python3
"""Render Google Benchmark JSON results and host hardware info as HTML.

Deliberately has no dependency on kora, CMake, or Doxygen: it only knows
about Google Benchmark's own JSON format and the host it's run on. Three
output formats share one underlying HTML fragment:

  fragment  the raw <h2>/<table> markup, meant to be embedded by something
            else (e.g. wrapped in a Doxygen page's doc comment).
  html      that fragment wrapped as a standalone, openable HTML document.
  dox       that fragment wrapped as a Doxygen @page -- a doc comment is
            fundamentally just that fragment with a comment syntax around
            it, so this is a thin convenience over the same content as the
            other two formats, not a separately maintained rendering path.
"""

import argparse
import html
import json
import math
import platform
import re
import statistics
import subprocess
from pathlib import Path

NS_PER_UNIT = {"ns": 1, "us": 1e3, "ms": 1e6, "s": 1e9}


def run(command):
    """Best-effort subprocess run: returns stdout, or None if unavailable."""
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=5, check=False)
    except (OSError, subprocess.SubprocessError):
        return None
    return result.stdout if result.returncode == 0 else None


def lscpu_field(output, label):
    if not output:
        return None
    match = re.search(rf"^{re.escape(label)}:\s*(.+)$", output, re.MULTILINE)
    return match.group(1).strip() if match else None


def gather_hardware_info():
    lscpu_output = run(["lscpu"])
    tuned_output = run(["tuned-adm", "active"])
    dmidecode_output = run(["dmidecode", "--type", "17"])

    info = {
        "CPU model": lscpu_field(lscpu_output, "Model name"),
        "Sockets": lscpu_field(lscpu_output, "Socket(s)"),
        "Cores per socket": lscpu_field(lscpu_output, "Core(s) per socket"),
        "Threads per core": lscpu_field(lscpu_output, "Thread(s) per core"),
        "Logical CPUs": lscpu_field(lscpu_output, "CPU(s)"),
        "Max CPU MHz": lscpu_field(lscpu_output, "CPU max MHz"),
        "Kernel": platform.release(),
    }

    if tuned_output:
        match = re.search(r"profile:\s*(.+)", tuned_output)
        info["tuned profile"] = match.group(1).strip() if match else tuned_output.strip()

    if dmidecode_output:
        speed_match = re.search(r"^\s*Speed:\s*(.+)$", dmidecode_output, re.MULTILINE)
        if speed_match and "Unknown" not in speed_match.group(1):
            info["Memory speed"] = speed_match.group(1).strip()

    return {key: value for key, value in info.items() if value}


def load_runs(json_paths):
    """One (executable name, context, benchmarks) tuple per readable file."""
    runs = []
    for path in json_paths:
        path = Path(path)
        if not path.is_file():
            continue
        try:
            data = json.loads(path.read_text())
        except json.JSONDecodeError:
            continue
        runs.append((path.stem, data.get("context", {}), data.get("benchmarks", [])))
    return runs


def to_ns(value, unit):
    return value * NS_PER_UNIT.get(unit, 1)


def percentile(sorted_values, pct):
    """Nearest-rank percentile; sorted_values must already be ascending."""
    if not sorted_values:
        return 0.0
    rank = max(1, math.ceil(pct / 100 * len(sorted_values)))
    return sorted_values[min(rank, len(sorted_values)) - 1]


def group_samples(benchmarks):
    """run_name -> {"iterations": <loop iterations/rep>, "samples": [(real_ns, cpu_ns), ...]}.

    Google Benchmark's own "aggregate" rows (mean/median/stddev/cv) are
    skipped -- only "iteration" rows are raw per-repetition samples, and
    computing percentiles ourselves from those is more useful than relying
    on the fixed set of aggregates Benchmark happens to compute. Each
    sample is itself already a per-iteration time: Benchmark runs the loop
    body many times within one repetition and divides by how many loops it
    took ("iterations", constant per run_name) to get a stable number, so
    what we aggregate across repetitions is already "time per iteration",
    not a total.
    """
    grouped = {}
    for entry in benchmarks:
        if entry.get("run_type", "iteration") != "iteration":
            continue
        name = entry.get("run_name", entry.get("name", ""))
        unit = entry.get("time_unit", "ns")
        group = grouped.setdefault(name, {"iterations": entry.get("iterations", 0), "samples": []})
        group["samples"].append((
            to_ns(entry.get("real_time", 0), unit),
            to_ns(entry.get("cpu_time", 0), unit),
        ))
    return grouped


def split_category(name):
    """"Group/Case" -> ("Group", "Case"); a bare name is its own category."""
    if "/" in name:
        category, _, case = name.partition("/")
        return category, case
    return name, name


def summarize(values):
    sorted_values = sorted(values)
    return {
        "n": len(sorted_values),
        "mean": statistics.mean(sorted_values) if sorted_values else 0.0,
        "p50": percentile(sorted_values, 50),
        "p99": percentile(sorted_values, 99),
    }


def format_time(ns):
    """ns, at whichever of ns/us/ms/s unit keeps the number readable."""
    if ns >= 1e9:
        return f"{ns / 1e9:,.3f} s"
    if ns >= 1e6:
        return f"{ns / 1e6:,.3f} ms"
    if ns >= 1e3:
        return f"{ns / 1e3:,.3f} µs"
    return f"{ns:,.1f} ns"


def render_table(headers, rows):
    lines = ["<table>", "<tr>" + "".join(f"<th>{html.escape(h)}</th>" for h in headers) + "</tr>"]
    for row in rows:
        lines.append("<tr>" + "".join(f"<td>{html.escape(str(cell))}</td>" for cell in row) + "</tr>")
    lines.append("</table>")
    return lines


def render_hardware_section(info):
    if not info:
        return []
    lines = ["<h2>Host</h2>"]
    lines += render_table(["Property", "Value"], info.items())
    return lines


def render_warning_banner(context):
    """A trust caveat, surfaced prominently rather than buried in a table.

    Both conditions directly affect whether the numbers below mean
    anything: scaling makes individual samples noisier, and a Debug build
    measures the compiler's debug code path, not what ships.
    """
    warnings = []
    if context.get("cpu_scaling_enabled"):
        warnings.append("CPU scaling is enabled: measurements may be noisy.")
    if context.get("library_build_type") == "debug":
        warnings.append("Built as Debug: timings may not reflect a Release build.")
    if not warnings:
        return []
    message = " ".join(warnings)
    return [
        '<div style="border:2px solid #d9534f;background:#fdf2f2;'
        'color:#a94442;padding:10px;border-radius:4px;margin:8px 0;">'
        f"<strong>&#9888; Warning:</strong> {html.escape(message)}</div>"
    ]


def render_run_section(name, context, benchmarks):
    lines = [f"<h2>{html.escape(name)}</h2>"]
    lines += render_warning_banner(context)

    categories = {}
    for run_name, group in group_samples(benchmarks).items():
        category, case = split_category(run_name)
        categories.setdefault(category, []).append((case, group))

    for category, cases in categories.items():
        lines.append(f"<h3>{html.escape(category)}</h3>")

        case_stats = {}
        repetitions = 0
        for case, group in cases:
            cpu_stats = summarize([cpu for _, cpu in group["samples"]])
            real_mean = statistics.mean([real for real, _ in group["samples"]]) if group["samples"] else 0.0
            case_stats[case] = {"cpu": cpu_stats, "real_mean": real_mean, "iterations": group["iterations"]}
            repetitions = cpu_stats["n"]

        fastest_mean = min((s["cpu"]["mean"] for s in case_stats.values()), default=0.0)
        lines.append(
            f"<p>Each cell is the per-iteration time (how long one call took, "
            f"amortized over the iterations/repetition below), aggregated "
            f"across n={repetitions} repetitions.</p>"
        )

        headers = ["Case", "Iterations/rep", "CPU mean", "CPU p50", "CPU p99", "Real mean", "Relative"]
        rows = []
        for case, stats in case_stats.items():
            cpu = stats["cpu"]
            relative = cpu["mean"] / fastest_mean if fastest_mean else 0.0
            rows.append([
                case,
                f"{stats['iterations']:,}",
                format_time(cpu["mean"]),
                format_time(cpu["p50"]),
                format_time(cpu["p99"]),
                format_time(stats["real_mean"]),
                f"{relative:,.2f}x",
            ])
        lines += render_table(headers, rows)
    return lines


def render_fragment(json_paths):
    """The shared HTML content every output format is built from."""
    lines = render_hardware_section(gather_hardware_info())
    for name, context, benchmarks in load_runs(json_paths):
        lines += render_run_section(name, context, benchmarks)
    return lines


def render_html(fragment_lines, title):
    return "\n".join([
        "<!doctype html>",
        '<html lang="en">',
        "<head>",
        '<meta charset="utf-8">',
        f"<title>{html.escape(title)}</title>",
        "</head>",
        "<body>",
        f"<h1>{html.escape(title)}</h1>",
        *fragment_lines,
        "</body>",
        "</html>",
    ])


def render_dox(fragment_lines, page_name, title):
    body = [f"@page {page_name} {title}", ""] + fragment_lines
    lines = ["/**"]
    for line in body:
        lines.append(f" * {line}" if line else " *")
    lines.append(" */")
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--format", choices=["fragment", "html", "dox"], default="fragment")
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--title", default="Benchmark Results")
    parser.add_argument("--page-name", default="benchmarks", help="@page name; only used with --format dox")
    parser.add_argument("json_files", nargs="*", type=Path)
    args = parser.parse_args()

    fragment_lines = render_fragment(args.json_files)

    if args.format == "fragment":
        content = "\n".join(fragment_lines)
    elif args.format == "html":
        content = render_html(fragment_lines, args.title)
    else:
        content = render_dox(fragment_lines, args.page_name, args.title)

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(content + "\n")


if __name__ == "__main__":
    main()
