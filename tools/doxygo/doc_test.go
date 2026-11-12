package main

import (
	"go/parser"
	"go/token"
	"testing"
)

/**
 * TestCommentText checks both comment styles reduce to the same text.
 */
func TestCommentText(t *testing.T) {
	cases := map[string]string{
		"line comments, directive dropped": "// Foo does X.\n//\n// @param a The a.\n//\n//go:noinline\n",
		"decorated block":                  "/**\n * Foo does X.\n *\n * @param a The a.\n */\n",
		"undecorated block":                "/*\n\tFoo does X.\n\n\t@param a The a.\n*/\n",
		"Doxygen-style line comments":      "/// Foo does X.\n///\n/// @param a The a.\n",
	}
	const want = "Foo does X.\n\n@param a The a."
	for name, comment := range cases {
		file, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\n\n"+comment+"func Foo(a int) {}\n", parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if got := commentText(file.Comments[0]); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}
