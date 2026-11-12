package render

import (
	"math"
	"testing"
)

/**
 * TestFormatTime checks unit selection, rounding and grouping.
 */
func TestFormatTime(t *testing.T) {
	for ns, want := range map[float64]string{
		7.24:            "7.2 ns",
		999.94:          "999.9 ns",
		1657:            "1.657 µs",
		39193.4:         "39.193 µs",
		2.5e6:           "2.500 ms",
		1234.5678e9:     "1,234.568 s",
		math.NaN():      "-",
		1_000_000_000.0: "1.000 s",
	} {
		if got := FormatTime(ns); got != want {
			t.Errorf("FormatTime(%v) = %q, want %q", ns, got, want)
		}
	}
}

/**
 * TestFormatNumbers checks thousands grouping, signs and non-finite values.
 */
func TestFormatNumbers(t *testing.T) {
	cases := []struct{ got, want string }{
		{FormatCount(82348860), "82,348,860"},
		{FormatCount(999), "999"},
		{FormatCount(-1000), "-1,000"},
		{FormatNumber(1234567.891, 2), "1,234,567.89"},
		{FormatNumber(math.NaN(), 2), "nan"},
		{FormatNumber(math.Inf(1), 2), "inf"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

/**
 * TestDoxygenMarkupText checks escaping and autolink suppression.
 */
func TestDoxygenMarkupText(t *testing.T) {
	m := DoxygenMarkup{}
	for in, want := range map[string]string{
		"BlazeBackend/Validate": "%BlazeBackend/%Validate",
		"7.2 ns":                "7.2 %ns",
		"1.657 µs":              "1.657 µs", // 's' follows a letter
		"1.20x":                 "1.20x",    // 'x' follows a digit
		"a<b & c":               "%a&lt;%b &amp; %c",
		"@cmd #x $y \\z":        "\\@%cmd \\#x \\$%y \\\\%z", // 'x' follows '#'
		"end */ here":           "%end *&#47; %here",
		"_private":              "%_private",
	} {
		if got := m.Text(in); got != want {
			t.Errorf("Text(%q) = %q, want %q", in, got, want)
		}
	}
}

/**
 * TestDoxygenMarkupLink checks labels are escaped but not autolink-guarded.
 */
func TestDoxygenMarkupLink(t *testing.T) {
	got := DoxygenMarkup{}.Link("doxythings_x", `a "b" */`)
	if want := `@ref doxythings_x "a &quot;b&quot; *&#47;"`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

/**
 * TestTable checks text cells are escaped and formatted cells aren't.
 */
func TestTable(t *testing.T) {
	table := NewTable("A", "B")
	table.AddRow(Text("<x>"), Formatted("<b>y</b>"))
	got := table.Lines(HTMLMarkup{})
	want := []string{
		`<table class="doxtable">`,
		"<tr><th>A</th><th>B</th></tr>",
		"<tr><td>&lt;x&gt;</td><td><b>y</b></td></tr>",
		"</table>",
	}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
