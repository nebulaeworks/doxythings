package link

import (
	"strings"
	"testing"

	"github.com/nebulaeworks/doxythings/internal/doxygen"
	"github.com/nebulaeworks/doxythings/internal/model"
)

/**
 * benchmark makes a Benchmark called name with the given run patterns.
 */
func benchmark(name string, patterns ...string) *Benchmark {
	return &Benchmark{Entity: &doxygen.Entity{RefID: name, Name: name}, RunPatterns: patterns}
}

/**
 * TestMatches checks exact and "pattern/" prefix matching.
 */
func TestMatches(t *testing.T) {
	b := benchmark("b", "A/B", "A")
	for name, want := range map[string]int{"A/B": 3, "A/B/8": 3, "A/C": 1, "A": 1, "AB": 0, "X": 0} {
		if got := b.Matches(name); got != want {
			t.Errorf("Matches(%q) = %d, want %d", name, got, want)
		}
	}
}

/**
 * TestLink checks the most specific benchmark wins, ties go to the first,
 * and unmatched results and unrun benchmarks are reported.
 */
func TestLink(t *testing.T) {
	general, specific, twin, unrun := benchmark("general", "A"), benchmark("specific", "A/B"),
		benchmark("twin", "A/B"), benchmark("unrun", "Z")
	suite := &model.Suite{Name: "s", Runs: []*model.Run{{Name: "A/B/1"}, {Name: "A/C"}, {Name: "Q"}}}
	r := Link([]*model.Suite{suite}, []*Benchmark{general, specific, twin, unrun})

	if got := r.BenchmarkOf(suite, suite.Runs[0]); got != specific {
		t.Errorf("A/B/1 went to %v", got.Entity.Name)
	}
	if got := r.BenchmarkOf(suite, suite.Runs[1]); got != general {
		t.Errorf("A/C went to %v", got.Entity.Name)
	}
	problems := strings.Join(r.Problems(), "\n")
	for _, want := range []string{"result 'Q' (s)", "twin: no results", "unrun: no results"} {
		if !strings.Contains(problems, want) {
			t.Errorf("problems lack %q:\n%s", want, problems)
		}
	}
	if len(r.Measured()) != 2 {
		t.Errorf("measured: %d", len(r.Measured()))
	}
}

/**
 * TestNilResult checks a nil Result behaves as "nothing linked".
 */
func TestNilResult(t *testing.T) {
	var r *Result
	if r.HasLinks() || r.BenchmarkOf(&model.Suite{}, &model.Run{}) != nil || r.Problems() != nil {
		t.Error("nil Result should find nothing")
	}
}
