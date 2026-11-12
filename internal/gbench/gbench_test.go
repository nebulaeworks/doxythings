package gbench

import (
	"os"
	"path/filepath"
	"testing"
)

/**
 * TestLoadFixture checks runs, units, host facts and skipped rows on the
 * end-to-end fixture.
 */
func TestLoadFixture(t *testing.T) {
	suite, err := Loader{}.Load(filepath.Join("..", "..", "testdata", "fixture", "results.json"))
	if err != nil {
		t.Fatal(err)
	}
	if suite.Name != "results" || suite.HostName != "fixture-host" {
		t.Errorf("got name %q, host %q", suite.Name, suite.HostName)
	}
	if len(suite.Runs) == 0 || suite.Runs[0].Name != "ToCoreJson/FlatArray" || len(suite.Runs[0].CPUNs) != 3 {
		t.Fatalf("unexpected runs: %+v", suite.Runs)
	}
	if v, _ := suite.Host.Get("Caches"); v != "L1 Data 32 KiB (shared by 2)" {
		t.Errorf("caches: %q", v)
	}
	if v, _ := suite.Host.Get("Load average"); v != "0.50, 0.40, 0.30" {
		t.Errorf("load average: %q", v)
	}
}

/**
 * TestLoadConvertsAndWarns checks unit conversion, aggregate rows being
 * skipped, and errors and Debug builds becoming warnings.
 */
func TestLoadConvertsAndWarns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	data := `{"context": {"library_build_type": "debug", "cpu_scaling_enabled": true},
	  "benchmarks": [
	    {"name": "A", "run_type": "iteration", "iterations": 5, "cpu_time": 2, "real_time": 3, "time_unit": "us"},
	    {"name": "A_mean", "run_name": "A", "run_type": "aggregate", "cpu_time": 99},
	    {"name": "B", "error_occurred": true, "error_message": "boom"}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	suite, err := Loader{}.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Runs) != 1 || suite.Runs[0].CPUNs[0] != 2000 || suite.Runs[0].RealNs[0] != 3000 {
		t.Errorf("runs: %+v", suite.Runs)
	}
	if len(suite.Warnings) != 3 || suite.Warnings[2] != "B failed: boom" {
		t.Errorf("warnings: %q", suite.Warnings)
	}
}

/**
 * TestLoadRejectsInvalidJSON checks a broken file is an error.
 */
func TestLoadRejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Loader{}).Load(path); err == nil {
		t.Error("expected an error")
	}
}
