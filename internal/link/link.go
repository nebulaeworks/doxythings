/**
 * Package link joins benchmark results to the documentation: which tagged
 * benchmark each result belongs to, and which entities each benchmark
 * measures.
 *
 * It sits between reading (packages gbench and doxygen) and rendering
 * (package render), so neither side knows about the other.
 */
package link

import (
	"strings"

	"github.com/nebulaeworks/doxythings/internal/doxygen"
	"github.com/nebulaeworks/doxythings/internal/model"
	"github.com/nebulaeworks/doxythings/internal/tags"
)

/**
 * Benchmark is a documented entity tagged as a benchmark.
 */
type Benchmark struct {
	/** Entity is the benchmark itself, e.g. its function. */
	Entity *doxygen.Entity
	/** RunPatterns are its `@benchmark_run` arguments. */
	RunPatterns []string
	/** Subjects are the entities its `@benchmarks` tags name. */
	Subjects []*doxygen.Entity
	/**
	 * UnresolvedSubjects are `@benchmarks` arguments Doxygen couldn't
	 * resolve to a documented entity.
	 */
	UnresolvedSubjects []string
}

/**
 * FindBenchmarks returns every benchmark tagged in index, in the order found.
 *
 * @param index The Doxygen index to search.
 * @return The benchmarks.
 */
func FindBenchmarks(index *doxygen.Index) []*Benchmark {
	var found []*Benchmark
	for _, entity := range index.Annotated() {
		a := entity.Annotations
		b := &Benchmark{Entity: entity, RunPatterns: a.Markers[tags.BenchmarkRun.Key]}
		for _, item := range a.XRefs[tags.Benchmarks.Key] {
			if item.RefID == "" {
				b.UnresolvedSubjects = append(b.UnresolvedSubjects, item.Text)
			} else if subject, ok := index.Lookup(item.RefID); ok {
				b.Subjects = append(b.Subjects, subject)
			}
		}
		if len(b.RunPatterns) > 0 || len(b.Subjects) > 0 || len(b.UnresolvedSubjects) > 0 {
			found = append(found, b)
		}
	}
	return found
}

/**
 * Matches returns the length of the longest of b's patterns that runName
 * matches, or 0 if none does. A pattern matches the name exactly, or as the
 * prefix of "pattern/...", so longer patterns are more specific.
 *
 * @param runName A result's run name.
 * @return The length of the matching pattern, or 0.
 */
func (b *Benchmark) Matches(runName string) int {
	best := 0
	for _, pattern := range b.RunPatterns {
		if runName == pattern || strings.HasPrefix(runName, pattern+"/") {
			best = max(best, len(pattern))
		}
	}
	return best
}

/**
 * Measurement is one run of one suite.
 */
type Measurement struct {
	/** Suite is the suite the run belongs to. */
	Suite *model.Suite
	/** Run is the measured case. */
	Run *model.Run
}

/**
 * SubjectRow is one result shown on a measured entity's page.
 */
type SubjectRow struct {
	/** Benchmark is the benchmark that produced it. */
	Benchmark *Benchmark
	/** Measurement is the result itself. */
	Measurement Measurement
}

/**
 * Subject is an entity measured by one or more benchmarks.
 */
type Subject struct {
	/** Entity is the measured entity. */
	Entity *doxygen.Entity
	/** Rows are its results, grouped by benchmark in benchmark order. */
	Rows []SubjectRow
}

/**
 * runKey identifies a run across suites.
 */
type runKey struct {
	/** suite is the suite's name. */
	suite string
	/** run is the run's name. */
	run string
}

/**
 * Result is the outcome of Link. A nil *Result means "no documentation to
 * link against": every lookup then finds nothing.
 */
type Result struct {
	/** benchmarks lists every benchmark, in the order found. */
	benchmarks []*Benchmark
	/** benchmarkOf maps each matched run to its benchmark. */
	benchmarkOf map[runKey]*Benchmark
	/** measured maps each benchmark to its runs, in result order. */
	measured map[*Benchmark][]Measurement
	/** problems lists everything that couldn't be matched. */
	problems []string
}

/**
 * Link matches every run in suites to the benchmark whose pattern matches it
 * most specifically (the first such benchmark on a tie).
 *
 * @param suites The results.
 * @param benchmarks The tagged benchmarks.
 * @return The matches, and what couldn't be matched.
 */
func Link(suites []*model.Suite, benchmarks []*Benchmark) *Result {
	r := &Result{
		benchmarks:  benchmarks,
		benchmarkOf: map[runKey]*Benchmark{},
		measured:    map[*Benchmark][]Measurement{},
	}
	for _, suite := range suites {
		for _, run := range suite.Runs {
			best, score := (*Benchmark)(nil), 0
			for _, b := range benchmarks {
				if s := b.Matches(run.Name); s > score {
					best, score = b, s
				}
			}
			if best == nil {
				r.problems = append(r.problems,
					"result '"+run.Name+"' ("+suite.Name+") matches no @"+tags.BenchmarkRun.Name+" tag")
				continue
			}
			r.benchmarkOf[runKey{suite.Name, run.Name}] = best
			r.measured[best] = append(r.measured[best], Measurement{suite, run})
		}
	}
	for _, b := range benchmarks {
		r.problems = append(r.problems, r.benchmarkProblems(b)...)
	}
	return r
}

/**
 * benchmarkProblems returns what's wrong with b's tags or results.
 *
 * @param b The benchmark to check.
 * @return One message per problem.
 */
func (r *Result) benchmarkProblems(b *Benchmark) []string {
	var problems []string
	name := b.Entity.Name
	for _, target := range b.UnresolvedSubjects {
		problems = append(problems, name+": @"+tags.Benchmarks.Name+" target '"+target+"' is not a documented entity")
	}
	switch {
	case len(b.RunPatterns) == 0:
		problems = append(problems, name+": has @"+tags.Benchmarks.Name+" but no @"+tags.BenchmarkRun.Name+" tag")
	case len(r.measured[b]) == 0:
		problems = append(problems, name+": no results for @"+tags.BenchmarkRun.Name+" "+strings.Join(b.RunPatterns, ", "))
	}
	return problems
}

/**
 * BenchmarkOf returns the benchmark a suite's run belongs to, or nil.
 *
 * @param suite The suite the run is in.
 * @param run The run.
 * @return The benchmark, or nil.
 */
func (r *Result) BenchmarkOf(suite *model.Suite, run *model.Run) *Benchmark {
	if r == nil {
		return nil
	}
	return r.benchmarkOf[runKey{suite.Name, run.Name}]
}

/**
 * Measured returns the benchmarks that have results, in the order found.
 *
 * @return The benchmarks with results.
 */
func (r *Result) Measured() []*Benchmark {
	if r == nil {
		return nil
	}
	var measured []*Benchmark
	for _, b := range r.benchmarks {
		if len(b.RunPatterns) > 0 && len(r.measured[b]) > 0 {
			measured = append(measured, b)
		}
	}
	return measured
}

/**
 * Measurements returns b's results, in result order.
 *
 * @param b The benchmark.
 * @return Its results.
 */
func (r *Result) Measurements(b *Benchmark) []Measurement {
	if r == nil {
		return nil
	}
	return r.measured[b]
}

/**
 * Subjects returns every entity measured by one of benchmarks, in the order
 * first named, with its rows from those benchmarks.
 *
 * @param benchmarks The benchmarks whose subjects to collect.
 * @return The measured entities.
 */
func (r *Result) Subjects(benchmarks []*Benchmark) []*Subject {
	var subjects []*Subject
	byID := map[string]*Subject{}
	for _, b := range benchmarks {
		for _, entity := range b.Subjects {
			s, seen := byID[entity.RefID]
			if !seen {
				s = &Subject{Entity: entity}
				byID[entity.RefID] = s
				subjects = append(subjects, s)
			}
			for _, m := range r.Measurements(b) {
				s.Rows = append(s.Rows, SubjectRow{b, m})
			}
		}
	}
	return subjects
}

/**
 * HasLinks reports whether any run was matched to a benchmark.
 *
 * @return Whether there is at least one match.
 */
func (r *Result) HasLinks() bool {
	return r != nil && len(r.benchmarkOf) > 0
}

/**
 * Problems returns everything that couldn't be matched, for reporting.
 *
 * @return One message per problem.
 */
func (r *Result) Problems() []string {
	if r == nil {
		return nil
	}
	return r.problems
}
