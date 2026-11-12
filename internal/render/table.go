package render

import "strings"

/**
 * Cell is one table cell: either text, escaped by the table's Markup, or
 * markup that is already formatted (such as a link).
 */
type Cell struct {
	/** content is the cell's text or markup. */
	content string
	/** isMarkup is true when content is already formatted. */
	isMarkup bool
}

/**
 * Text makes a cell holding plain text.
 *
 * @param value The text.
 * @return The cell.
 */
func Text(value string) Cell {
	return Cell{content: value}
}

/**
 * Formatted makes a cell holding already-formatted markup.
 *
 * @param markup The cell's markup.
 * @return The cell.
 */
func Formatted(markup string) Cell {
	return Cell{content: markup, isMarkup: true}
}

/**
 * Table is an HTML table using Doxygen's table style.
 */
type Table struct {
	/** headers are the column headings. */
	headers []string
	/** rows are the body rows. */
	rows [][]Cell
}

/**
 * NewTable starts a table with the given column headings.
 *
 * @param headers The column headings.
 * @return The empty table.
 */
func NewTable(headers ...string) *Table {
	return &Table{headers: headers}
}

/**
 * AddRow appends a body row.
 *
 * @param cells The row's cells, one per column.
 */
func (t *Table) AddRow(cells ...Cell) {
	t.rows = append(t.rows, cells)
}

/**
 * AddTextRow appends a body row of plain-text cells.
 *
 * @param values The row's text, one per column.
 */
func (t *Table) AddTextRow(values ...string) {
	t.AddRow(textCells(values)...)
}

/**
 * Lines writes the table as HTML lines, escaping text cells with markup.
 *
 * @param markup How to escape text cells.
 * @return The table's HTML lines.
 */
func (t *Table) Lines(markup Markup) []string {
	lines := []string{`<table class="doxtable">`, t.row("th", textCells(t.headers), markup)}
	for _, row := range t.rows {
		lines = append(lines, t.row("td", row, markup))
	}
	return append(lines, "</table>")
}

/**
 * row writes one row, with each cell in a tag element.
 *
 * @param tag The cell element, th or td.
 * @param cells The row's cells.
 * @param markup How to escape text cells.
 * @return The row's HTML.
 */
func (t *Table) row(tag string, cells []Cell, markup Markup) string {
	var b strings.Builder
	b.WriteString("<tr>")
	for _, c := range cells {
		content := c.content
		if !c.isMarkup {
			content = markup.Text(content)
		}
		b.WriteString("<" + tag + ">" + content + "</" + tag + ">")
	}
	b.WriteString("</tr>")
	return b.String()
}

/**
 * textCells makes a plain-text cell of each value.
 *
 * @param values The texts.
 * @return The cells.
 */
func textCells(values []string) []Cell {
	cells := make([]Cell, len(values))
	for i, v := range values {
		cells[i] = Text(v)
	}
	return cells
}
