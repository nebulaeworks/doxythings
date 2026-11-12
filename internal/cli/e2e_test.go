package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

/**
 * pipeline is the result of running the whole two-pass flow on
 * testdata/fixture with the real Doxygen.
 */
type pipeline struct {
	/** html is the final HTML output directory. */
	html string
	/** exitCode is what "doxythings doxygen" returned. */
	exitCode int
	/** stderr is what "doxythings doxygen" reported. */
	stderr string
	/**
	 * pass1Warnings and pass2Warnings are Doxygen's warnings from each pass.
	 */
	pass1Warnings, pass2Warnings string
}

/**
 * runPipeline copies the fixture into a scratch directory and runs: init,
 * an XML-only Doxygen pass, "doxythings doxygen", and the final Doxygen pass.
 */
func runPipeline(t *testing.T) *pipeline {
	t.Helper()
	if _, err := exec.LookPath("doxygen"); err != nil {
		t.Skip("doxygen not installed")
	}
	work := t.TempDir()
	if err := os.CopyFS(work, os.DirFS(filepath.Join("..", "..", "testdata", "fixture"))); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"generated", "fragments"} {
		if err := os.Mkdir(filepath.Join(work, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	conf := filepath.Join(work, "conf")
	if code := NewApp(&bytes.Buffer{}, os.Stderr, "test").Run([]string{"init", "--dir", conf}); code != ExitOK {
		t.Fatalf("init exited %d", code)
	}

	p := &pipeline{html: filepath.Join(work, "out", "html")}
	p.pass1Warnings = runDoxygen(t, work, "GENERATE_HTML = NO", "GENERATE_XML = YES")
	var stderr bytes.Buffer
	p.exitCode = NewApp(&bytes.Buffer{}, &stderr, "test").Run([]string{
		"doxygen",
		"--output", filepath.Join(work, "generated", "benchmarks.dox"),
		"--fragments-dir", filepath.Join(work, "fragments"),
		"--xml-dir", filepath.Join(work, "out", "xml"),
		filepath.Join(work, "results.json"),
	})
	p.stderr = stderr.String()
	if err := os.RemoveAll(filepath.Join(work, "out")); err != nil {
		t.Fatal(err)
	}
	p.pass2Warnings = runDoxygen(t, work)
	return p
}

/**
 * runDoxygen runs Doxygen in work on the fixture's Doxyfile, doxythings'
 * fragment and overrides, and returns its warnings.
 */
func runDoxygen(t *testing.T, work string, overrides ...string) string {
	t.Helper()
	config := strings.Join(append([]string{
		"@INCLUDE = " + filepath.Join(work, "Doxyfile"),
		"@INCLUDE = " + filepath.Join(work, "conf", "doxythings.doxyfile"),
	}, overrides...), "\n")
	cmd := exec.Command("doxygen", "-")
	cmd.Dir = work
	cmd.Stdin = strings.NewReader(config)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("doxygen: %v\n%s", err, out)
	}
	log, err := os.ReadFile(filepath.Join(work, "out", "warnings.log"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(log))
}

/**
 * page returns one generated HTML page.
 */
func (p *pipeline) page(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(p.html, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

/**
 * TestEndToEnd checks the documentation the whole flow produces.
 */
func TestEndToEnd(t *testing.T) {
	p := runPipeline(t)

	t.Run("both passes are warning free", func(t *testing.T) {
		if p.pass1Warnings != "" || p.pass2Warnings != "" {
			t.Errorf("pass 1:\n%s\npass 2:\n%s", p.pass1Warnings, p.pass2Warnings)
		}
	})

	t.Run("reports unmatched results and benchmarks", func(t *testing.T) {
		if p.exitCode != ExitOK {
			t.Errorf("exit code %d, stderr:\n%s", p.exitCode, p.stderr)
		}
		for _, want := range []string{"'Unmatched/Result'", "bench::NeverRun: no results"} {
			if !strings.Contains(p.stderr, want) {
				t.Errorf("stderr lacks %q:\n%s", want, p.stderr)
			}
		}
	})

	t.Run("benchmark shows its results, always expanded", func(t *testing.T) {
		group := p.page(t, "group__bench__group.html")
		for _, want := range []string{`id="doxythings_`, "BlazeBackend/Validate/8", "BlazeBackend/Validate/64", "<dt>Results</dt>"} {
			if !strings.Contains(group, want) {
				t.Errorf("benchmark page lacks %q", want)
			}
		}
		if strings.Contains(group, "<details") {
			t.Error("benchmark results should not be collapsible")
		}
	})

	t.Run("subjects show a collapsible Performance section linking back", func(t *testing.T) {
		section := regexp.MustCompile(`<details\s+class="doxythings"\s*>\s*<summary\s*>\s*<b>Performance</b>`)
		for page, runs := range map[string][]string{
			"namespacelib_1_1detail.html":  {"ToCoreJson/FlatArray", "BlazeBackend/Validate/8"},
			"classlib_1_1BasicSchema.html": {"BlazeBackend/Validate/64"},
		} {
			content := p.page(t, page)
			if !section.MatchString(content) {
				t.Errorf("%s: no collapsible Performance section", page)
			}
			for _, run := range runs {
				if !linkTo(`doxythings_[^"]*`, run).MatchString(content) {
					t.Errorf("%s: no link to %s's benchmark", page, run)
				}
			}
		}
	})

	t.Run("stylesheet is installed", func(t *testing.T) {
		if _, err := os.Stat(filepath.Join(p.html, "doxythings.css")); err != nil {
			t.Error(err)
		}
	})

	t.Run("overview links runs and subjects", func(t *testing.T) {
		overview := p.page(t, "benchmarks.html")
		if !linkTo(`doxythings_[^"]*`, "ToCoreJson/FlatArray").MatchString(overview) {
			t.Error("run not linked to its benchmark")
		}
		if !linkTo(`doxythings_subject_[^"]*`, "lib::detail::to_core_json").MatchString(overview) {
			t.Error("subject not linked")
		}
		if !strings.Contains(overview, "Unmatched/Result") {
			t.Error("unmatched result missing")
		}
	})
}

/**
 * linkTo matches an HTML link to an anchor matching anchorPattern, showing
 * label.
 */
func linkTo(anchorPattern, label string) *regexp.Regexp {
	return regexp.MustCompile(`href="[^"]*#` + anchorPattern + `"[^>]*>` + regexp.QuoteMeta(label) + `<`)
}
