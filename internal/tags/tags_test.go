package tags

import (
	"strings"
	"testing"
)

/**
 * TestAliases checks each form's Doxygen alias.
 */
func TestAliases(t *testing.T) {
	for tag, want := range map[string]string{
		BenchmarkRun.Alias(): `ALIASES += benchmark_run{1}="\xmlonly<doxythings-benchmark-run>\1</doxythings-benchmark-run>\endxmlonly"`,
		Benchmarks.Alias():   `ALIASES += benchmarks{1}="\xrefitem doxythings_benchmarks \"Benchmarks\" \"Benchmark Index\" \ref \1"`,
	} {
		if tag != want {
			t.Errorf("got  %s\nwant %s", tag, want)
		}
	}
}

/**
 * TestVocabulary checks the set recognises only its own markers and keys.
 */
func TestVocabulary(t *testing.T) {
	if !All.IsMarker("doxythings-benchmark-run") || All.IsMarker("para") {
		t.Error("IsMarker")
	}
	if !All.IsXRefKey("doxythings_benchmarks") || All.IsXRefKey("todo") {
		t.Error("IsXRefKey")
	}
	if !strings.Contains(All.Doxyfile(), BenchmarkRun.Alias()) {
		t.Error("Doxyfile lacks an alias")
	}
}
