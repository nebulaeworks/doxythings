package main

import (
	"fmt"
	"hash/fnv"
	"strings"
)

/**
 * maxGraphNodes caps call and caller graphs, as Doxygen's
 * DOT_GRAPH_MAX_NODES does.
 */
const maxGraphNodes = 50

/**
 * labelWrap is the line length node labels wrap at, as Doxygen's
 * DOT_WRAP_THRESHOLD.
 */
const labelWrap = 17

/**
 * crossReferences returns what doxygo adds to the documentation of the
 * symbol called name, in place of what Doxygen would derive from C++ code:
 * its "References" and "Referenced by" lists and, for a function, its call
 * and caller graphs. The markup is on one line, so the documentation
 * doesn't grow longer than the comment it came from.
 *
 * @param name The symbol's C++ name.
 * @return The Doxygen markup, or "" if there is nothing to add.
 */
func (x *symbolIndex) crossReferences(name string) string {
	sym := x.lookup(name)
	if sym == nil {
		return ""
	}
	var parts []string
	if list := x.referenceList("References", name, sym.Refs); list != "" {
		parts = append(parts, list)
	}
	if list := x.referenceList("Referenced by", name, sym.Callers); list != "" {
		parts = append(parts, list)
	}
	if sym.Kind == kindFunction {
		if graph := x.graphSection(name, false); graph != "" {
			parts = append(parts, graph)
		}
		if graph := x.graphSection(name, true); graph != "" {
			parts = append(parts, graph)
		}
	}
	return strings.Join(parts, " ")
}

/**
 * referenceList returns a paragraph linking to targets, styled like
 * Doxygen's own: "References a(), b(), and c()."
 *
 * @param label The paragraph's first words.
 * @param from The C++ name of the symbol being documented.
 * @param targets The C++ names to link to.
 * @return The paragraph, or "" for no targets.
 */
func (x *symbolIndex) referenceList(label, from string, targets []string) string {
	if len(targets) == 0 {
		return ""
	}
	links := make([]string, len(targets))
	for i, target := range targets {
		links[i] = fmt.Sprintf(`@ref %s "%s"`, target, x.displayName(target, from))
	}
	text := links[0]
	if len(links) > 1 {
		text = strings.Join(links[:len(links)-1], ", ") + ", and " + links[len(links)-1]
	}
	// A plain <p>: Doxygen gives a <p>'s attributes to the paragraph
	// before it.
	return "<p>" + label + " " + text + ".</p>"
}

/**
 * displayName returns target as Doxygen would show it in from's
 * documentation: without the scope the two share, and with "()" if it is a
 * function.
 *
 * @param target The C++ name shown.
 * @param from The C++ name of the symbol being documented.
 * @return The name to show.
 */
func (x *symbolIndex) displayName(target, from string) string {
	shown := target
	scope := scopeOf(from)
	namespace := ""
	if sym := x.lookup(from); sym != nil {
		namespace = sym.Namespace
	}
	for scope != "" && len(scope) >= len(namespace) {
		if rest, ok := strings.CutPrefix(target, scope+"::"); ok {
			shown = rest
			break
		}
		scope = scopeOf(scope)
	}
	if sym := x.lookup(target); sym != nil && sym.Kind == kindFunction {
		shown += "()"
	}
	return shown
}

/**
 * scopeOf returns the scope a C++ name is declared in, "" for none.
 *
 * @param name The C++ name.
 * @return Its scope.
 */
func scopeOf(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[:i]
	}
	return ""
}

/**
 * graph returns the functions reachable from name by following calls (or,
 * with callers, calls made to it) breadth first, at most maxGraphNodes of
 * them with name first, and the calls between them as pairs of indexes.
 *
 * @param name The function's C++ name.
 * @param callers Whether to follow callers instead of callees.
 * @return The nodes' C++ names, and the edges between them.
 */
func (x *symbolIndex) graph(name string, callers bool) ([]string, [][2]int) {
	nodes := []string{name}
	index := map[string]int{name: 0}
	var edges [][2]int
	for i := 0; i < len(nodes); i++ {
		sym := x.lookup(nodes[i])
		next := sym.Refs
		if callers {
			next = sym.Callers
		}
		for _, target := range next {
			if s := x.lookup(target); s == nil || s.Kind != kindFunction {
				continue
			}
			j, seen := index[target]
			if !seen {
				if len(nodes) == maxGraphNodes {
					continue
				}
				j = len(nodes)
				index[target] = j
				nodes = append(nodes, target)
			}
			edges = append(edges, [2]int{i, j})
		}
	}
	return nodes, edges
}

/**
 * graphSection returns a function's call graph (or caller graph) in a
 * collapsible section like Doxygen's own, or "" if the function calls (or
 * is called by) nothing in the module. The empty caption after "@dot"
 * keeps Doxygen from reading the start of a one-line graph as one.
 *
 * @param name The function's C++ name.
 * @param callers Whether to draw the caller graph instead of the call graph.
 * @return The section's markup, or "".
 */
func (x *symbolIndex) graphSection(name string, callers bool) string {
	nodes, edges := x.graph(name, callers)
	if len(edges) == 0 {
		return ""
	}
	title, kind, direction, edgeDir := "Here is the call graph for this function:", "cg", "LR", ""
	if callers {
		title, kind, direction, edgeDir = "Here is the caller graph for this function:", "icg", "RL", `dir="back",`
	}
	var dot strings.Builder
	dot.WriteString(`digraph "` + name + `" { bgcolor="transparent"; ` +
		`edge [fontname=Helvetica,fontsize=10,labelfontname=Helvetica,labelfontsize=10]; ` +
		`node [fontname=Helvetica,fontsize=10,shape=box,height=0.2,width=0.4]; ` +
		`rankdir="` + direction + `"; `)
	for i, node := range nodes {
		style := `color="grey40", fillcolor="white", style="filled", URL="\ref ` + node + `"`
		if i == 0 {
			style = `color="gray40", fillcolor="grey60", style="filled", fontcolor="black"`
		}
		fmt.Fprintf(&dot, `Node%d [label="%s", %s, tooltip="%s"]; `,
			i+1, wrapLabel(node), style, dotEscape(x.lookup(node).Brief))
	}
	for _, e := range edges {
		fmt.Fprintf(&dot, `Node%d -> Node%d [%scolor="steelblue1",style="solid"]; `, e[0]+1, e[1]+1, edgeDir)
	}
	dot.WriteString("}")

	id := fmt.Sprintf("dynsection-doxygo-%x-%s", hashOf(name), kind)
	return `@htmlonly<div id="` + id + `" class="dynheader closed"><span class="dynarrow">` +
		`<span class="arrowhead closed"></span></span>` + title + `</div>` +
		`<div id="` + id + `-summary" class="dynsummary"></div>` +
		`<div id="` + id + `-content" class="dyncontent hidden">@endhtmlonly ` +
		`@dot "" ` + dot.String() + ` @enddot @htmlonly</div>@endhtmlonly`
}

/**
 * wrapLabel breaks a C++ name into left-aligned lines of about labelWrap
 * characters, at "::" boundaries, as Doxygen labels graph nodes.
 *
 * @param name The C++ name.
 * @return The DOT label.
 */
func wrapLabel(name string) string {
	parts := strings.Split(name, "::")
	var lines []string
	line := parts[0]
	for _, part := range parts[1:] {
		if len(line)+2+len(part) > labelWrap {
			lines = append(lines, line)
			line = "::" + part
		} else {
			line += "::" + part
		}
	}
	lines = append(lines, line)
	if len(lines) == 1 {
		return lines[0]
	}
	return strings.Join(lines, `\l`) + `\l`
}

/**
 * dotEscape makes text safe inside a quoted DOT string within a C++
 * comment: quotes and backslashes are escaped, and comment delimiters
 * spaced out.
 *
 * @param text The text.
 * @return The escaped text.
 */
func dotEscape(text string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "*/", "* /", "/*", "/ *").Replace(text)
}

/**
 * hashOf returns a short stable hash of text, for HTML ids.
 *
 * @param text The text.
 * @return The hash.
 */
func hashOf(text string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(text))
	return h.Sum32()
}
