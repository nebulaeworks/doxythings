/**
 * Package gbench loads Google Benchmark's JSON output
 * (--benchmark_out_format=json) into model.Suites.
 */
package gbench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nebulaeworks/doxythings/internal/model"
)

/**
 * nsPerUnit converts Google Benchmark's time units to nanoseconds.
 */
var nsPerUnit = map[string]float64{"ns": 1, "us": 1e3, "ms": 1e6, "s": 1e9}

/**
 * Loader reads Google Benchmark result files. Its zero value is ready to use.
 */
type Loader struct{}

/**
 * Load reads one result file into a Suite named after the file.
 *
 * @param path The Google Benchmark JSON file.
 * @return The suite, or an error if the file can't be read or parsed.
 */
func (Loader) Load(path string) (*model.Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return doc.suite(name), nil
}

/**
 * document is the top level of a result file.
 */
type document struct {
	/** Context describes the machine and library the benchmarks ran with. */
	Context context `json:"context"`
	/** Benchmarks holds one entry per repetition, plus aggregate rows. */
	Benchmarks []entry `json:"benchmarks"`
}

/**
 * context is the file's "context" object. Numbers are kept as written, so
 * they display exactly as the harness reported them.
 */
type context struct {
	/** HostName is the machine's network name. */
	HostName string `json:"host_name"`
	/** Date is when the run started. */
	Date string `json:"date"`
	/** NumCPUs is the number of logical CPUs. */
	NumCPUs json.Number `json:"num_cpus"`
	/** MHzPerCPU is the CPU clock the library measured. */
	MHzPerCPU json.Number `json:"mhz_per_cpu"`
	/**
	 * CPUScalingEnabled is true when frequency scaling may have added noise.
	 */
	CPUScalingEnabled bool `json:"cpu_scaling_enabled"`
	/** LoadAvg is the 1, 5 and 15 minute load average at the start. */
	LoadAvg []float64 `json:"load_avg"`
	/** Caches describes the CPU caches. */
	Caches []cache `json:"caches"`
	/** LibraryBuildType is "release" or "debug": how the library was built. */
	LibraryBuildType string `json:"library_build_type"`
}

/**
 * cache describes one CPU cache.
 */
type cache struct {
	/** Type is e.g. "Data", "Instruction" or "Unified". */
	Type string `json:"type"`
	/** Level is the cache level (1, 2, 3...), nil if not reported. */
	Level *int `json:"level"`
	/** Size is the cache's size in bytes. */
	Size int64 `json:"size"`
	/** NumSharing is how many logical CPUs share it. */
	NumSharing int `json:"num_sharing"`
}

/**
 * entry is one row of the "benchmarks" array. Pointer fields distinguish
 * "absent" from an empty value, where the format gives absent a default.
 */
type entry struct {
	/** Name is the row's full name, including any aggregate suffix. */
	Name string `json:"name"`
	/** RunName is the case's name, shared by all its repetitions. */
	RunName *string `json:"run_name"`
	/** RunType is "iteration" for a measured repetition or "aggregate". */
	RunType *string `json:"run_type"`
	/** Iterations is how many times the body ran in this repetition. */
	Iterations int64 `json:"iterations"`
	/** RealTime is the per-iteration wall-clock time, in TimeUnit. */
	RealTime float64 `json:"real_time"`
	/** CPUTime is the per-iteration CPU time, in TimeUnit. */
	CPUTime float64 `json:"cpu_time"`
	/** TimeUnit is "ns", "us", "ms" or "s"; nanoseconds when absent. */
	TimeUnit *string `json:"time_unit"`
	/** ErrorOccurred is true when the benchmark reported a failure. */
	ErrorOccurred bool `json:"error_occurred"`
	/** ErrorMessage says why it failed. */
	ErrorMessage *string `json:"error_message"`
}

/**
 * suite converts the document into a Suite called name.
 *
 * @param name The suite's name.
 * @return The suite.
 */
func (d *document) suite(name string) *model.Suite {
	s := &model.Suite{Name: name, Host: d.Context.facts(), HostName: d.Context.HostName}
	if d.Context.CPUScalingEnabled {
		s.Warnings = append(s.Warnings, "CPU frequency scaling was enabled: measurements may be noisy.")
	}
	if d.Context.LibraryBuildType == "debug" {
		s.Warnings = append(s.Warnings, "The Google Benchmark library was built as Debug.")
	}

	runs := map[string]*model.Run{}
	for _, e := range d.Benchmarks {
		// Aggregate rows (mean/median/stddev/cv) are the library's own
		// summary of the iteration rows; ours are computed from the samples.
		if valueOr(e.RunType, "iteration") != "iteration" {
			continue
		}
		runName := valueOr(e.RunName, e.Name)
		if e.ErrorOccurred {
			s.Warnings = append(s.Warnings,
				fmt.Sprintf("%s failed: %s", runName, valueOr(e.ErrorMessage, "unknown error")))
			continue
		}
		scale, known := nsPerUnit[valueOr(e.TimeUnit, "ns")]
		if !known {
			scale = 1
		}
		run, seen := runs[runName]
		if !seen {
			run = &model.Run{Name: runName, Iterations: e.Iterations}
			runs[runName] = run
			s.Runs = append(s.Runs, run)
		}
		run.RealNs = append(run.RealNs, e.RealTime*scale)
		run.CPUNs = append(run.CPUNs, e.CPUTime*scale)
	}
	return s
}

/**
 * facts turns the context into display-ready host facts, leaving out any the
 * file doesn't report.
 *
 * @return The facts, in display order.
 */
func (c *context) facts() model.Facts {
	loads := make([]string, len(c.LoadAvg))
	for i, load := range c.LoadAvg {
		loads[i] = fmt.Sprintf("%.2f", load)
	}
	all := model.Facts{
		{Label: "Host", Value: c.HostName},
		{Label: "Run at", Value: c.Date},
		{Label: "CPUs", Value: c.NumCPUs.String()},
		{Label: "MHz per CPU", Value: c.MHzPerCPU.String()},
		{Label: "Caches", Value: formatCaches(c.Caches)},
		{Label: "Load average", Value: strings.Join(loads, ", ")},
		{Label: "Benchmark library build", Value: c.LibraryBuildType},
	}
	var reported model.Facts
	for _, fact := range all {
		if fact.Value != "" {
			reported = append(reported, fact)
		}
	}
	return reported
}

/**
 * formatCaches describes caches on one line, e.g.
 * "L1 Data 32 KiB (shared by 2), L3 Unified 16 MiB".
 *
 * @param caches The caches the file reports.
 * @return The description.
 */
func formatCaches(caches []cache) string {
	parts := make([]string, len(caches))
	for i, c := range caches {
		size := fmt.Sprintf("%d KiB", c.Size/1024)
		if c.Size >= 1024*1024 {
			size = fmt.Sprintf("%d MiB", c.Size/1024/1024)
		}
		level := "?"
		if c.Level != nil {
			level = fmt.Sprint(*c.Level)
		}
		parts[i] = fmt.Sprintf("L%s %s %s", level, c.Type, size)
		if c.NumSharing > 1 {
			parts[i] += fmt.Sprintf(" (shared by %d)", c.NumSharing)
		}
	}
	return strings.Join(parts, ", ")
}

/**
 * valueOr returns *p, or fallback when p is nil.
 *
 * @param p The value, or nil.
 * @param fallback What to return when p is nil.
 * @return The value or fallback.
 */
func valueOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}
