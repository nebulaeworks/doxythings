package render

import (
	"math"
	"strconv"
	"strings"

	"github.com/nebulaeworks/doxythings/internal/link"
	"github.com/nebulaeworks/doxythings/internal/model"
)

/**
 * perSuiteFacts are host facts that differ between executables of the same
 * run, so they're shown per suite rather than in the shared Host table.
 */
var perSuiteFacts = []string{"Run at", "Load average"}

/**
 * Overview is the all-results content: host facts, then one table per
 * benchmark group of each suite. Both the Doxygen page and the standalone
 * page are built from it.
 */
type Overview struct {
	/** Suites are the results to show. */
	Suites []*model.Suite
	/**
	 * HostDetails are extra facts about the current machine, added to the
	 * Host table.
	 */
	HostDetails model.Facts
	/** Links says where each run and subject is documented; nil for none. */
	Links *link.Result
}

/**
 * Lines writes the overview as HTML lines using markup.
 *
 * @param markup How to escape text and write links.
 * @return The overview's HTML lines.
 */
func (o *Overview) Lines(markup Markup) []string {
	var lines []string
	if host := o.hostFacts(); len(host) > 0 {
		lines = append(lines, "<h2>Host</h2>")
		table := NewTable("Property", "Value")
		for _, fact := range host {
			table.AddTextRow(fact.Label, fact.Value)
		}
		lines = append(lines, table.Lines(markup)...)
	}
	for _, suite := range o.Suites {
		lines = append(lines, o.suiteLines(suite, markup)...)
	}
	return append(lines, "<p>Each time is per iteration (one call, averaged over that repetition's "+
		"iterations), summarised across n repetitions. Relative compares CPU means within a group.</p>")
}

/**
 * hostFacts returns the shared Host table's facts: the first suite's, minus
 * the per-suite ones, plus HostDetails.
 *
 * @return The facts.
 */
func (o *Overview) hostFacts() model.Facts {
	var host model.Facts
	if len(o.Suites) > 0 {
		host = o.Suites[0].Host.Without(perSuiteFacts...)
	}
	for _, fact := range o.HostDetails {
		host.Set(fact.Label, fact.Value)
	}
	return host
}

/**
 * suiteLines writes one suite: its warnings, its per-suite facts, and a
 * table for each of its groups.
 *
 * @param suite The suite.
 * @param markup How to escape text and write links.
 * @return The suite's HTML lines.
 */
func (o *Overview) suiteLines(suite *model.Suite, markup Markup) []string {
	lines := []string{"<h2>" + markup.Text(suite.Name) + "</h2>"}
	lines = append(lines, warningBanner(suite.Warnings, markup)...)
	var facts []string
	for _, label := range perSuiteFacts {
		if value, ok := suite.Host.Get(label); ok {
			facts = append(facts, label+": "+value)
		}
	}
	if len(facts) > 0 {
		lines = append(lines, "<p>"+markup.Text(strings.Join(facts, "; "))+"</p>")
	}
	for _, group := range groupRuns(suite.Runs) {
		lines = append(lines, "<h3>"+markup.Text(group.name)+"</h3>")
		lines = append(lines, o.groupTable(suite, group.runs, markup).Lines(markup)...)
	}
	return lines
}

/**
 * groupTable builds the table for one group of a suite's runs.
 *
 * @param suite The suite the runs are in.
 * @param runs The group's runs.
 * @param markup How to write links.
 * @return The table.
 */
func (o *Overview) groupTable(suite *model.Suite, runs []*model.Run, markup Markup) *Table {
	showSubjects := o.Links.HasLinks()
	headers := []string{"Run"}
	if showSubjects {
		headers = append(headers, "Measures")
	}
	headers = append(headers, "n", "Iterations/rep")
	headers = append(headers, timeColumns...)
	table := NewTable(append(headers, "Relative")...)

	fastest := math.NaN()
	for _, run := range runs {
		if mean := run.CPU().Mean; math.IsNaN(fastest) || mean < fastest {
			fastest = mean
		}
	}
	for _, run := range runs {
		benchmark := o.Links.BenchmarkOf(suite, run)
		row := []Cell{runCell(benchmark, run, markup)}
		if showSubjects {
			row = append(row, subjectsCell(benchmark, markup))
		}
		cpu := run.CPU()
		relative := math.NaN()
		if fastest != 0 {
			relative = cpu.Mean / fastest
		}
		row = append(row, Text(strconv.Itoa(cpu.N)), Text(FormatCount(run.Iterations)))
		row = append(row, textCells(timeCells(run))...)
		table.AddRow(append(row, Text(FormatNumber(relative, 2)+"x"))...)
	}
	return table
}

/**
 * runCell shows a run's name, linked to its benchmark's docs if it has one.
 *
 * @param benchmark The run's benchmark, or nil.
 * @param run The run.
 * @param markup How to write links.
 * @return The cell.
 */
func runCell(benchmark *link.Benchmark, run *model.Run, markup Markup) Cell {
	if benchmark == nil {
		return Text(run.Name)
	}
	return Formatted(markup.Link(benchmarkAnchor(benchmark), run.Name))
}

/**
 * subjectsCell links to every entity a run's benchmark measures.
 *
 * @param benchmark The run's benchmark, or nil.
 * @param markup How to write links.
 * @return The cell.
 */
func subjectsCell(benchmark *link.Benchmark, markup Markup) Cell {
	if benchmark == nil {
		return Formatted("")
	}
	links := make([]string, len(benchmark.Subjects))
	for i, s := range benchmark.Subjects {
		links[i] = markup.Link(subjectAnchor(s.RefID), s.Name)
	}
	return Formatted(strings.Join(links, ", "))
}

/**
 * runGroup is the runs sharing one group name.
 */
type runGroup struct {
	/** name is the group's name (see model.Run.Group). */
	name string
	/** runs are its runs, in result order. */
	runs []*model.Run
}

/**
 * groupRuns splits runs by group, keeping the order groups first appear in.
 *
 * @param runs The runs, in result order.
 * @return The groups.
 */
func groupRuns(runs []*model.Run) []*runGroup {
	var groups []*runGroup
	byName := map[string]*runGroup{}
	for _, run := range runs {
		g, seen := byName[run.Group()]
		if !seen {
			g = &runGroup{name: run.Group()}
			byName[g.name] = g
			groups = append(groups, g)
		}
		g.runs = append(g.runs, run)
	}
	return groups
}

/**
 * warningBanner shows messages in a red box, or nothing if there are none.
 *
 * @param messages The warnings.
 * @param markup How to escape the messages.
 * @return The banner's HTML lines.
 */
func warningBanner(messages []string, markup Markup) []string {
	if len(messages) == 0 {
		return nil
	}
	escaped := make([]string, len(messages))
	for i, m := range messages {
		escaped[i] = markup.Text(m)
	}
	return []string{`<div style="border:2px solid #d9534f;background:#fdf2f2;color:#a94442;` +
		`padding:10px;border-radius:4px;margin:8px 0;"><strong>&#9888; Warning:</strong> ` +
		strings.Join(escaped, " ") + "</div>"}
}
