/**
 * Package model holds benchmark results in a form independent of any
 * harness or output format.
 *
 * Every input format produces Suites (see package gbench), and every renderer
 * reads only Suites, so supporting another harness means writing one loader
 * without touching any renderer.
 */
package model

import "strings"

/**
 * Run is one benchmark case, measured over one or more repetitions.
 *
 * Each sample is already a per-iteration time: harnesses run the body many
 * times per repetition and divide by the iteration count.
 */
type Run struct {
	/**
	 * Name is the harness's name for the case, e.g. "BlazeBackend/Validate/8".
	 */
	Name string
	/** Iterations is how many times one repetition ran the benchmark body. */
	Iterations int64
	/** RealNs holds one wall-clock time per repetition, in nanoseconds. */
	RealNs []float64
	/** CPUNs holds one CPU time per repetition, in nanoseconds. */
	CPUNs []float64
}

/**
 * Group returns the part of the name before the first "/", so that
 * "Group/Case/..." belongs to "Group"; a name with no "/" is its own group.
 *
 * @return The group's name.
 */
func (r *Run) Group() string {
	group, _, _ := strings.Cut(r.Name, "/")
	return group
}

/**
 * CPU summarises the CPU time across repetitions.
 *
 * @return The CPU time statistics, in nanoseconds.
 */
func (r *Run) CPU() Stats {
	return StatsOf(r.CPUNs)
}

/**
 * Real summarises the wall-clock time across repetitions.
 *
 * @return The wall-clock time statistics, in nanoseconds.
 */
func (r *Run) Real() Stats {
	return StatsOf(r.RealNs)
}

/**
 * Suite is everything one benchmark executable reported.
 *
 * Host and Warnings are already display-ready: only the loader knows what its
 * harness's context fields mean, so it does the interpreting.
 */
type Suite struct {
	/** Name identifies the suite, usually its result file's base name. */
	Name string
	/** Runs lists the cases in the order the harness reported them. */
	Runs []*Run
	/** Host describes the machine the suite ran on, in display order. */
	Host Facts
	/**
	 * HostName is the machine's network name, used to tell whether the
	 * current machine is the one the suite ran on.
	 */
	HostName string
	/** Warnings lists anything that makes the results less trustworthy. */
	Warnings []string
}

/**
 * Fact is one labelled piece of information, such as "CPUs: 8".
 */
type Fact struct {
	/** Label names the fact, e.g. "CPUs". */
	Label string
	/** Value is the fact's display text. */
	Value string
}

/**
 * Facts is an ordered list of Fact, kept in the order it should be shown.
 */
type Facts []Fact

/**
 * Get returns the value labelled label, and whether there is one.
 *
 * @param label The fact's label.
 * @return The value, and whether there is one.
 */
func (f Facts) Get(label string) (string, bool) {
	for _, fact := range f {
		if fact.Label == label {
			return fact.Value, true
		}
	}
	return "", false
}

/**
 * Set replaces the value labelled label, or appends it if there is none.
 *
 * @param label The fact's label.
 * @param value Its new value.
 */
func (f *Facts) Set(label, value string) {
	for i := range *f {
		if (*f)[i].Label == label {
			(*f)[i].Value = value
			return
		}
	}
	*f = append(*f, Fact{label, value})
}

/**
 * Without returns the facts whose label is not among labels.
 *
 * @param labels The labels to leave out.
 * @return The remaining facts, in their original order.
 */
func (f Facts) Without(labels ...string) Facts {
	var kept Facts
	for _, fact := range f {
		excluded := false
		for _, label := range labels {
			excluded = excluded || fact.Label == label
		}
		if !excluded {
			kept = append(kept, fact)
		}
	}
	return kept
}
