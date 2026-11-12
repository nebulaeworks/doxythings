/**
 * Command doxygo is a Doxygen INPUT_FILTER that lets Doxygen document Go.
 *
 * Doxygen has no Go parser, so doxygo rewrites one Go file into C++-shaped
 * declarations Doxygen's C++ parser understands, keeping every doc comment:
 *
 *   - a package becomes a namespace named after its directory (e.g.
 *     doxythings::internal::render), documented by its package comment;
 *   - a struct becomes a struct (embedded fields become base classes), an
 *     interface a class of pure virtual methods, and any other named type
 *     with methods a class; other named types become "using" aliases;
 *   - functions, methods, constants and variables keep their names, with Go
 *     types spelled the C++ way: *T as T*, []T as slice<T>, pkg.T as
 *     pkg::T, map[K]V as map<K, V>, several results as tuple<...>.
 *
 * Each declaration is written on the line it occupies in the Go source, so
 * Doxygen's line numbers and source links point at the real code.
 *
 * Usage, in a Doxyfile:
 *
 * 	EXTENSION_MAPPING = go=C++
 * 	FILTER_PATTERNS   = *.go=path/to/doxygo
 */
package main

import (
	"fmt"
	"os"
)

/**
 * main filters the file named by the only argument to standard output.
 */
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: doxygo <file.go>")
		os.Exit(2)
	}
	out, err := Filter(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "doxygo:", err)
		os.Exit(1)
	}
	os.Stdout.WriteString(out)
}
