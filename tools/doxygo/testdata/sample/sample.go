// Package sample exercises every translation doxygo makes.
package sample

import "strings"

/**
 * Shape is anything with an area.
 */
type Shape interface {
	/**
	 * Area returns the area.
	 *
	 * @return The area.
	 */
	Area() float64
}

// base is embedded in Square.
type base struct{}

// Square is a [Shape] with equal sides.
type Square struct {
	base
	// Side is the side length.
	Side  float64
	names []string // names are the square's labels.
	/** tags are more labels. */
	tags  []string
	count int /**< count is how many labels there are. */
}

// Area implements Shape.
func (s *Square) Area() float64 { return s.Side * s.Side }

// Names lists labels; see /* not a comment */ here.
type Names []string

// Join joins the names with sep, as [strings.Join] does.
//
// @param sep What goes between names.
// @return The joined names.
//
//go:noinline
func (n Names) Join(sep string) string { return strings.Join(n, sep) }

// Mode picks a behaviour.
type Mode int

// Modes.
const (
	// Fast is fast.
	Fast Mode = iota
	// Name is a name, with <b>bold</b> HTML.
	Name = "x"
)

/*
Lookup finds a square, or reports false; see [Square.Area].

@param byName The squares.
@param visit Called on each square.
@param template Unused.
@param names The squares to look up.
@return The square, and whether it was found.
*/
func Lookup(byName map[string]*Square, visit func(*Square) error, template string, names ...string) (*Square, bool) {
	var s Square
	return &s, s.Area() > 0
}
