package main

import (
	"go/ast"
	"regexp"
	"strconv"
	"strings"
)

/**
 * commentText returns the text of a doc comment without its comment
 * markers. Both styles work, and can be mixed in one group:
 *
 *   - line comments: "//" and one space are removed from each line ("///"
 *     and "//!" too, as Doxygen allows), and directives such as
 *     "//go:generate" are dropped;
 *   - block comments: the opening marker (slash-star, with a second star
 *     or a "!" for Doxygen, and a "<" for a trailing comment) and the
 *     closing one are removed, as is a leading "*" on each line if every
 *     line has one, and then the indentation all lines share.
 *
 * Leading and trailing blank lines are dropped.
 *
 * @param doc The comment, or nil.
 * @return The text.
 */
func commentText(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	var lines []string
	for _, c := range doc.List {
		if body, ok := strings.CutPrefix(c.Text, "//"); ok {
			if isDirective(body) {
				continue
			}
			body = strings.TrimLeft(body, "/!")
			lines = append(lines, strings.TrimPrefix(body, " "))
		} else {
			lines = append(lines, blockCommentLines(c.Text)...)
		}
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

/**
 * isDirective reports whether the text after "//" is a tool directive
 * rather than documentation, following go/ast: "go:generate",
 * "nolint:all", "line file.go:10" and the like.
 *
 * @param body The comment's text after "//".
 * @return Whether it is a directive.
 */
func isDirective(body string) bool {
	if strings.HasPrefix(body, "line ") || strings.HasPrefix(body, "extern ") || strings.HasPrefix(body, "export ") {
		return true
	}
	colon := strings.Index(body, ":")
	if colon <= 0 || colon+1 >= len(body) {
		return false
	}
	for _, r := range body[:colon] {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return false
		}
	}
	r := body[colon+1]
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
}

/**
 * blockCommentLines returns the lines of a block comment without its
 * markers, decoration or shared indentation.
 *
 * @param text The comment, markers included.
 * @return Its lines.
 */
func blockCommentLines(text string) []string {
	text = strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
	if strings.HasPrefix(text, "*") || strings.HasPrefix(text, "!") {
		text = text[1:]
	}
	text = strings.TrimPrefix(text, "<")
	lines := strings.Split(text, "\n")
	lines[0] = strings.TrimPrefix(lines[0], " ")
	rest := lines[1:]
	decorated := len(rest) > 0
	for _, line := range rest {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "*") {
			decorated = false
		}
	}
	if decorated {
		for i, line := range rest {
			line = strings.TrimPrefix(strings.TrimLeft(line, " \t"), "*")
			rest[i] = strings.TrimPrefix(line, " ")
		}
		return lines
	}
	indent := sharedIndent(rest)
	for i, line := range rest {
		rest[i] = strings.TrimPrefix(line, indent)
	}
	return lines
}

/**
 * sharedIndent returns the leading whitespace every non-blank line has.
 *
 * @param lines The lines.
 * @return The shared indentation.
 */
func sharedIndent(lines []string) string {
	indent, first := "", true
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if first {
			indent, first = lead, false
			continue
		}
		for !strings.HasPrefix(lead, indent) {
			indent = indent[:len(indent)-1]
		}
	}
	return indent
}

/**
 * docLink matches a Go doc link: [Name], [Name.Member], [pkg.Name] or
 * [pkg.Name.Member], optionally starred, not followed by what would make
 * it a Markdown link instead.
 */
var docLink = regexp.MustCompile(`\[\*?([A-Za-z_]\w*(?:\.[A-Za-z_]\w*){0,2})\]([^(\[:]|$)`)

/**
 * paramName matches a parameter documented with `@param` (or `\param`, with
 * an optional direction), whose name is renamed like the parameter itself
 * if it is a C++ keyword.
 */
var paramName = regexp.MustCompile(`([@\\]param(?:\[[a-z,]+\])?\s+)(\w+)`)

/**
 * docWriter turns the doc comments of one file into Doxygen comment
 * blocks. The text is passed to Doxygen as is, so Doxygen commands and
 * Markdown work as in C++; Go doc links are turned into Doxygen links.
 */
type docWriter struct {
	/** module is the module the file belongs to. */
	module *module
	/** namespace is the C++ namespace of the file's package. */
	namespace string
	/**
	 * index holds the module's symbols, to resolve doc links; nil
	 * resolves none.
	 */
	index *symbolIndex
	/** imports maps the package names the file imports to import paths. */
	imports map[string]string
}

/**
 * newDocWriter returns a docWriter for file, of the package with the C++
 * namespace given.
 *
 * @param m The module the file belongs to.
 * @param namespace The C++ namespace of the file's package.
 * @param index The module's symbols, or nil.
 * @param file The parsed file, for its imports.
 * @return The docWriter.
 */
func newDocWriter(m *module, namespace string, index *symbolIndex, file *ast.File) *docWriter {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
	}
	return &docWriter{module: m, namespace: namespace, index: index, imports: imports}
}

/**
 * block returns doc as a Doxygen block comment, starting with command (such as
 * "@namespace name", or "") and ending with extra (doxygo's additions, or
 * "") on its last line, so the block takes no more lines than the comment.
 * It returns "" if there is nothing to write.
 *
 * @param doc The comment, or nil.
 * @param command A Doxygen command to start with, or "".
 * @param extra Markup to end with, or "".
 * @return The comment block, or "".
 */
func (w *docWriter) block(doc *ast.CommentGroup, command, extra string) string {
	text := w.text(doc)
	if text == "" && command == "" && extra == "" {
		return ""
	}
	return "/** " + joinNonEmpty(command, text, extra) + " */"
}

/**
 * trailing returns doc, a comment after a member, as a trailing Doxygen
 * block comment on one line, followed by extra.
 *
 * @param doc The comment.
 * @param extra Markup to end with, or "".
 * @return The comment block.
 */
func (w *docWriter) trailing(doc *ast.CommentGroup, extra string) string {
	text := strings.Join(strings.Fields(w.text(doc)), " ")
	return "/**< " + joinNonEmpty(text, extra) + " */"
}

/**
 * text returns a doc comment's text with its Go doc links resolved and
 * comment delimiters made harmless.
 *
 * @param doc The comment, or nil.
 * @return The Doxygen text.
 */
func (w *docWriter) text(doc *ast.CommentGroup) string {
	lines := strings.Split(commentText(doc), "\n")
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			inFence = !inFence
		}
		if !inFence && !isCodeLine(line) {
			line = paramName.ReplaceAllStringFunc(w.resolveLinks(line), func(match string) string {
				sub := paramName.FindStringSubmatch(match)
				return sub[1] + cppName(sub[2])
			})
		}
		lines[i] = breakCommentEnd(line)
	}
	return strings.Join(lines, "\n")
}

/**
 * isCodeLine reports whether a line is indented code, which Go doc and
 * Doxygen's Markdown both show verbatim.
 *
 * @param line The line.
 * @return Whether it is code.
 */
func isCodeLine(line string) bool {
	return strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ")
}

/**
 * resolveLinks turns the Go doc links on a line, outside `code` spans,
 * into Doxygen references (for the module's own symbols) or Markdown links
 * to pkg.go.dev (for other packages). Links that resolve to nothing are
 * left as written, as Go doc does.
 *
 * @param line The line.
 * @return The line with links resolved.
 */
func (w *docWriter) resolveLinks(line string) string {
	parts := strings.Split(line, "`")
	for i := 0; i < len(parts); i += 2 {
		parts[i] = docLink.ReplaceAllStringFunc(parts[i], func(match string) string {
			sub := docLink.FindStringSubmatch(match)
			if link := w.resolve(sub[1]); link != "" {
				return link + sub[2]
			}
			return match
		})
	}
	return strings.Join(parts, "`")
}

/**
 * resolve returns the link for a doc link's target, written as in Go
 * ("Name", "Type.Method", "pkg.Name" or "pkg.Type.Method"), or "".
 *
 * @param target The link's target.
 * @return The link's markup, or "".
 */
func (w *docWriter) resolve(target string) string {
	parts := strings.Split(target, ".")
	// [Name] and [Type.Member] in this package.
	if len(parts) <= 2 {
		if name := w.symbolName(w.namespace, parts); name != "" {
			return `@ref ` + name + ` "` + target + `"`
		}
	}
	// [pkg.Name] and [pkg.Type.Member] in an imported package.
	path, ok := w.imports[parts[0]]
	if !ok || len(parts) == 1 {
		return ""
	}
	if w.module != nil && w.module.contains(path) {
		if name := w.symbolName(w.module.namespace(path), parts[1:]); name != "" {
			return `@ref ` + name + ` "` + target + `"`
		}
		return ""
	}
	return "[" + target + "](https://pkg.go.dev/" + path + "#" + strings.Join(parts[1:], ".") + ")"
}

/**
 * symbolName returns the C++ name of the symbol named by parts within
 * namespace, if the index knows it.
 *
 * @param namespace The C++ namespace to look in.
 * @param parts The Go name's parts, e.g. a type and a method.
 * @return The C++ name, or "".
 */
func (w *docWriter) symbolName(namespace string, parts []string) string {
	names := make([]string, len(parts))
	for i, part := range parts {
		names[i] = cppName(part)
	}
	name := namespace + "::" + strings.Join(names, "::")
	if w.index.lookup(name) == nil {
		return ""
	}
	return name
}

/**
 * joinNonEmpty joins the parts that aren't empty with spaces.
 *
 * @param parts The strings to join.
 * @return The joined text.
 */
func joinNonEmpty(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " ")
}

/**
 * escapeDoc escapes the characters Doxygen would treat as commands,
 * references or HTML, for text doxygo generates rather than copies.
 *
 * @param text The text.
 * @return The escaped text.
 */
func escapeDoc(text string) string {
	var b strings.Builder
	for _, r := range text {
		if strings.ContainsRune(`\@<>&#%$`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

/**
 * breakCommentEnd keeps a closing comment marker (star-slash) from ending
 * the generated comment, and an opening one from looking like a nested
 * one, by writing their slash as an entity.
 *
 * @param text The text.
 * @return The text, safe inside a comment.
 */
func breakCommentEnd(text string) string {
	text = strings.ReplaceAll(text, "*/", "*&#47;")
	return strings.ReplaceAll(text, "/*", "&#47;*")
}
