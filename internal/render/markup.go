package render

import (
	"strings"
	"unicode"
)

/**
 * Markup decides how text and links are written for one output format, so
 * that all formats share one set of page builders and can't drift apart.
 */
type Markup interface {
	/**
	 * Text returns value made safe to appear as plain text.
	 *
	 * @param value The text.
	 * @return The escaped text.
	 */
	Text(value string) string
	/**
	 * Link returns label as a link to the Doxygen anchor called anchor, or
	 * as plain text where links aren't available.
	 *
	 * @param anchor The Doxygen anchor or entity to link to.
	 * @param label The text to show.
	 * @return The link, or the label alone.
	 */
	Link(anchor, label string) string
}

/**
 * HTMLMarkup writes plain HTML, without links.
 */
type HTMLMarkup struct{}

/**
 * Text escapes value for HTML.
 *
 * @param value The text.
 * @return The escaped text.
 */
func (HTMLMarkup) Text(value string) string {
	return escapeHTML(value, true)
}

/**
 * Link returns the label as text: a standalone page has nothing to link to.
 *
 * @param label The text to show.
 * @return The escaped label.
 */
func (m HTMLMarkup) Link(_, label string) string {
	return m.Text(label)
}

/**
 * DoxygenMarkup writes text safe inside a Doxygen block comment, with
 * Doxygen's autolinking turned off.
 *
 * Benchmark names often contain class names (e.g. "BlazeBackend/Validate"),
 * which Doxygen would otherwise turn into unrelated links; a '%' before a
 * word stops that.
 */
type DoxygenMarkup struct{}

/**
 * Text escapes value for HTML, then for Doxygen's commands, comment
 * terminators and autolinking.
 *
 * @param value The text.
 * @return The escaped text.
 */
func (DoxygenMarkup) Text(value string) string {
	value = escapeHTML(value, false)
	var b strings.Builder
	for _, r := range value {
		if strings.ContainsRune(`\@#$`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	value = strings.ReplaceAll(b.String(), "*/", "*&#47;")
	return suppressAutolinks(value)
}

/**
 * Link writes a Doxygen reference to anchor, showing label.
 *
 * @param anchor The Doxygen anchor or entity to link to.
 * @param label The text to show.
 * @return The reference.
 */
func (DoxygenMarkup) Link(anchor, label string) string {
	label = strings.ReplaceAll(escapeHTML(label, true), "*/", "*&#47;")
	return "@ref " + anchor + ` "` + label + `"`
}

/**
 * suppressAutolinks puts '%' before every identifier-like word that doesn't
 * directly follow a word character, '%', '&' or '#' (so "&amp;" and
 * "&#47;" stay intact).
 *
 * @param value Text already escaped for HTML.
 * @return The text with autolinking suppressed.
 */
func suppressAutolinks(value string) string {
	runes := []rune(value)
	var b strings.Builder
	for i := 0; i < len(runes); {
		r := runes[i]
		startsWord := r == '_' || (r < unicode.MaxASCII && unicode.IsLetter(r))
		if startsWord && (i == 0 || !blocksAutolinkPrefix(runes[i-1])) {
			b.WriteByte('%')
			for i < len(runes) && isWordRune(runes[i]) {
				b.WriteRune(runes[i])
				i++
			}
			continue
		}
		b.WriteRune(r)
		i++
	}
	return b.String()
}

/**
 * blocksAutolinkPrefix reports whether a word right after r must be left
 * alone: it continues a word, or is part of an escape or entity.
 *
 * @param r The character before the word.
 * @return Whether to leave the word alone.
 */
func blocksAutolinkPrefix(r rune) bool {
	return isWordRune(r) || r == '%' || r == '&' || r == '#'
}

/**
 * isWordRune reports whether r can be part of a word (letters, digits and
 * underscore, in any script).
 *
 * @param r The character.
 * @return Whether it is a word character.
 */
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

/**
 * escapeHTML escapes &, < and >, plus both quotes when quote is true.
 *
 * @param value The text.
 * @param quote Whether to escape quotes too.
 * @return The escaped text.
 */
func escapeHTML(value string, quote bool) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r == '&':
			b.WriteString("&amp;")
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case quote && r == '"':
			b.WriteString("&quot;")
		case quote && r == '\'':
			b.WriteString("&#x27;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
