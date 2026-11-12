package model

import (
	"math"
	"testing"
)

/**
 * TestStatsOf checks each statistic, including even-length medians.
 */
func TestStatsOf(t *testing.T) {
	s := StatsOf([]float64{4, 1, 3, 2})
	if s.N != 4 || s.Mean != 2.5 || s.Median != 2.5 || s.Min != 1 || s.Max != 4 {
		t.Errorf("got %+v", s)
	}
	if want := math.Sqrt(5.0 / 3.0); math.Abs(s.StdDev-want) > 1e-12 {
		t.Errorf("stddev %v, want %v", s.StdDev, want)
	}
}

/**
 * TestStatsOfEdgeCases checks no samples and a single sample.
 */
func TestStatsOfEdgeCases(t *testing.T) {
	if s := StatsOf(nil); !s.IsEmpty() || !math.IsNaN(s.Median) {
		t.Errorf("empty: got %+v", s)
	}
	if s := StatsOf([]float64{7}); s.StdDev != 0 || s.Median != 7 {
		t.Errorf("single: got %+v", s)
	}
}

/**
 * TestExactSum checks the sum doesn't lose small values to rounding.
 */
func TestExactSum(t *testing.T) {
	if got := exactSum([]float64{1e16, 1, -1e16}); got != 1 {
		t.Errorf("got %v, want 1", got)
	}
}

/**
 * TestRunGroup checks group names come from before the first "/".
 */
func TestRunGroup(t *testing.T) {
	for name, want := range map[string]string{"A/B/8": "A", "Plain": "Plain"} {
		if got := (&Run{Name: name}).Group(); got != want {
			t.Errorf("%s: got %q", name, got)
		}
	}
}

/**
 * TestFacts checks Set replaces in place and Without filters.
 */
func TestFacts(t *testing.T) {
	f := Facts{{"A", "1"}, {"B", "2"}}
	f.Set("A", "3")
	f.Set("C", "4")
	if v, _ := f.Get("A"); v != "3" || len(f) != 3 || f[0].Label != "A" {
		t.Errorf("got %v", f)
	}
	if kept := f.Without("A", "C"); len(kept) != 1 || kept[0].Label != "B" {
		t.Errorf("got %v", kept)
	}
}
