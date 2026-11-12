package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/**
 * TestFilter checks each kind of translation on testdata/sample.
 */
func TestFilter(t *testing.T) {
	path := filepath.Join("testdata", "sample", "sample.go")
	out, err := Filter(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"/** @namespace doxythings::tools::doxygo::testdata::sample Package sample exercises",
		"namespace doxythings::tools::doxygo::testdata::sample {",
		"#include <strings>",
		// Both comment styles, Doxygen commands passed through.
		"/** Shape is anything with an area. ",
		"@return The area.",
		"class Shape { public:",
		"virtual float64 Area() = 0;",
		// Implemented interfaces become bases, and "X implements Y" docs copies.
		"struct Square : public base, public Shape {",
		"/** @copydoc doxythings::tools::doxygo::testdata::sample::Shape::Area",
		"float64 Side;",
		"private: slice<string> names; /**< names are the square's labels.",
		"/** tags are more labels.",
		"int count; /**< count is how many labels there are.",
		"float64 Area(); };",
		"float64 Square::Area() {",
		"Go type: <tt>type Names []string</tt>",
		"class Names { public: string Join(string sep); };",
		"string Names::Join(string sep) {",
		"using Mode = int;",
		"const Mode Fast;",
		`const auto Name = "x";`,
		"with <b>bold</b> HTML.",
		"see &#47;* not a comment *&#47; here.",
		// Go doc links.
		`a @ref doxythings::tools::doxygo::testdata::sample::Shape "Shape" with`,
		"[strings.Join](https://pkg.go.dev/strings#Join)",
		`see @ref doxythings::tools::doxygo::testdata::sample::Square::Area "Square.Area".`,
		// Parameters named like C++ keywords are renamed in @param too.
		"@param template_ Unused.",
		"tuple<Square*, bool> Lookup(map<string, Square*> byName, func<error(Square*)> visit, string template_, string... names)",
		// References, found by the type checker.
		`<p>References @ref doxythings::tools::doxygo::testdata::sample::Square::Area "Square::Area()".</p>`,
		`<p>Referenced by @ref doxythings::tools::doxygo::testdata::sample::Lookup "Lookup()".</p>`,
		"Here is the call graph for this function:",
		"Here is the caller graph for this function:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	if t.Failed() {
		t.Log(out)
	}
}

/**
 * TestFilterKeepsLines checks declarations stay on their source lines.
 */
func TestFilterKeepsLines(t *testing.T) {
	path := filepath.Join("testdata", "sample", "sample.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Filter(path)
	if err != nil {
		t.Fatal(err)
	}
	lineOf := func(text, needle string) int {
		return strings.Count(text[:strings.Index(text, needle)], "\n")
	}
	if got, want := lineOf(out, "float64 Square::Area() {"), lineOf(string(source), "func (s *Square) Area"); got != want {
		t.Errorf("method on line %d, want %d", got+1, want+1)
	}
	if got, want := lineOf(out, "tuple<Square*, bool> Lookup("), lineOf(string(source), "func Lookup"); got != want {
		t.Errorf("function on line %d, want %d", got+1, want+1)
	}
}
