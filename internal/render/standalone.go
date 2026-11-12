package render

import (
	"strings"

	"github.com/nebulaeworks/doxythings/internal/model"
)

/**
 * StandaloneRenderer writes the overview as a self-contained HTML page,
 * needing no Doxygen.
 */
type StandaloneRenderer struct {
	/** Title is the page's title and heading. */
	Title string
}

/**
 * Render returns the page's HTML.
 *
 * @param suites The results.
 * @param hostDetails Extra facts about the machine, shown with the host table.
 * @return The HTML.
 */
func (r *StandaloneRenderer) Render(suites []*model.Suite, hostDetails model.Facts) string {
	markup := HTMLMarkup{}
	title := markup.Text(r.Title)
	lines := []string{
		"<!doctype html>", `<html lang="en">`, "<head>", `<meta charset="utf-8">`,
		"<title>" + title + "</title>",
		"<style>body{font-family:sans-serif;margin:2em}table{border-collapse:collapse;margin:1em 0}" +
			"th,td{border:1px solid #ccc;padding:4px 8px;text-align:right}" +
			"td:first-child,th:first-child{text-align:left}</style>",
		"</head>", "<body>", "<h1>" + title + "</h1>",
	}
	overview := &Overview{Suites: suites, HostDetails: hostDetails}
	lines = append(lines, overview.Lines(markup)...)
	lines = append(lines, "</body>", "</html>", "")
	return strings.Join(lines, "\n")
}
