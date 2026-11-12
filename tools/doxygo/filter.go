package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"regexp"
	"strconv"
	"strings"
)

/**
 * Filter returns the C++-shaped translation of the Go file at path.
 *
 * @param path The Go file.
 * @return The translation, or an error if the file or its package can't be read
 * or parsed.
 */
func Filter(path string) (string, error) {
	pkg, err := loadPackage(path)
	if err != nil {
		return "", err
	}
	file, err := parser.ParseFile(pkg.fset, path, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}
	index, err := cacheFromEnv().load(pkg.module, pkg.module.dir(pkg.path))
	if err != nil {
		return "", err
	}
	t := &translator{
		pkg:   pkg,
		out:   &lineWriter{fset: pkg.fset, line: 1},
		docs:  newDocWriter(pkg.module, pkg.namespace, index, file),
		index: index,
	}
	t.file(file)
	return t.out.String(), nil
}

/**
 * lineWriter builds output while tracking its line number, so text can be
 * placed on the line it came from.
 */
type lineWriter struct {
	/** fset maps source positions to lines. */
	fset *token.FileSet
	/** b holds the output so far. */
	b strings.Builder
	/** line is the output line being written, starting at 1. */
	line int
}

/**
 * moveTo starts new lines until the output reaches pos's source line. If
 * the output is already past it, writing just continues where it is.
 *
 * @param pos The source position to reach.
 */
func (w *lineWriter) moveTo(pos token.Pos) {
	for target := w.fset.Position(pos).Line; w.line < target; w.line++ {
		w.b.WriteByte('\n')
	}
}

/**
 * atLineStart reports whether nothing has been written on the current
 * line yet.
 *
 * @return Whether the line is empty so far.
 */
func (w *lineWriter) atLineStart() bool {
	text := w.b.String()
	return text == "" || strings.HasSuffix(text, "\n")
}

/**
 * write appends text, counting the lines it adds.
 *
 * @param text The text to append.
 */
func (w *lineWriter) write(text string) {
	w.b.WriteString(text)
	w.line += strings.Count(text, "\n")
}

/**
 * String returns the output, ending with a newline.
 *
 * @return The output.
 */
func (w *lineWriter) String() string {
	return w.b.String() + "\n"
}

/**
 * translator writes one Go file as C++-shaped declarations.
 */
type translator struct {
	/** pkg is the package the file belongs to. */
	pkg *goPackage
	/** out receives the translation. */
	out *lineWriter
	/** docs turns doc comments into Doxygen comment blocks. */
	docs *docWriter
	/** index holds the module's symbols and the references between them. */
	index *symbolIndex
}

/**
 * file translates a whole file: file and package comments, imports as
 * includes, then the declarations in a namespace.
 *
 * @param f The parsed file.
 */
func (t *translator) file(f *ast.File) {
	fileDoc := t.fileComment(f)
	if fileDoc != nil {
		t.writeDoc(fileDoc, "", "")
	}
	if f.Doc != nil {
		t.writeDoc(f.Doc, "@namespace "+t.pkg.namespace, "")
	}
	t.out.moveTo(f.Package)
	if fileDoc == nil {
		// Every file gets a page, as headers do in C++, for its include
		// graph and its list of declarations.
		t.out.write("/** @file */ ")
	}
	t.imports(f)
	// The namespace opens after the includes: Doxygen's preprocessor
	// pastes in files included inside braces.
	t.out.moveTo(firstDeclaration(f))
	t.out.write("namespace " + t.pkg.namespace + " { ")
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			t.genDecl(d)
		case *ast.FuncDecl:
			t.funcDecl(d)
		}
	}
	t.out.write("\n}")
}

/**
 * firstDeclaration returns where the first declaration after the imports
 * starts (its doc comment, if it has one), or the end of the file.
 *
 * @param f The parsed file.
 * @return The position.
 */
func firstDeclaration(f *ast.File) token.Pos {
	for _, decl := range f.Decls {
		if d, ok := decl.(*ast.GenDecl); ok && d.Tok == token.IMPORT {
			continue
		}
		var doc *ast.CommentGroup
		switch d := decl.(type) {
		case *ast.GenDecl:
			doc = d.Doc
			if doc == nil && len(d.Specs) > 0 {
				switch s := d.Specs[0].(type) {
				case *ast.TypeSpec:
					doc = s.Doc
				case *ast.ValueSpec:
					doc = s.Doc
				}
			}
		case *ast.FuncDecl:
			doc = d.Doc
		}
		if doc != nil {
			return doc.Pos()
		}
		return decl.Pos()
	}
	return f.FileEnd
}

/**
 * fileComment returns the comment documenting the file itself: one before
 * the package clause, apart from the package comment, starting with
 * "@file" (or "\file"). Go ignores such a comment.
 *
 * @param f The parsed file.
 * @return The comment, or nil.
 */
func (t *translator) fileComment(f *ast.File) *ast.CommentGroup {
	for _, c := range f.Comments {
		if c.Pos() >= f.Package {
			break
		}
		text := commentText(c)
		if c != f.Doc && (strings.HasPrefix(text, "@file") || strings.HasPrefix(text, `\file`)) {
			return c
		}
	}
	return nil
}

/**
 * imports writes each import as an include, on its own line: of the file
 * holding the imported package's comment for packages of the module, so
 * include graphs and directory graphs show package dependencies, and of
 * the import path for others, as system headers are shown in C++.
 *
 * @param f The parsed file.
 */
func (t *translator) imports(f *ast.File) {
	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		t.out.moveTo(spec.Pos())
		if !t.out.atLineStart() {
			continue // an include must start its line; never shift lines for one
		}
		if t.pkg.module.contains(path) {
			if file := t.pkg.module.docFile(path); file != "" {
				t.out.write(`#include "` + file + `"`)
				continue
			}
		}
		t.out.write("#include <" + path + ">")
	}
}

/**
 * genDecl translates a type, const or var declaration; imports are dropped.
 *
 * @param d The declaration.
 */
func (t *translator) genDecl(d *ast.GenDecl) {
	for _, spec := range d.Specs {
		var doc *ast.CommentGroup
		switch s := spec.(type) {
		case *ast.TypeSpec:
			doc = s.Doc
		case *ast.ValueSpec:
			doc = s.Doc
		}
		if doc == nil && len(d.Specs) == 1 {
			doc = d.Doc
		}
		switch s := spec.(type) {
		case *ast.TypeSpec:
			t.typeSpec(s, doc)
		case *ast.ValueSpec:
			t.valueSpec(d.Tok, s, doc)
		}
	}
}

/**
 * writeDoc places doc on its own source lines, starting with command and
 * ending with extra; it writes nothing if all three are empty.
 *
 * @param doc The comment, or nil.
 * @param command A Doxygen command to start with, or "".
 * @param extra Markup to end with, or "".
 */
func (t *translator) writeDoc(doc *ast.CommentGroup, command, extra string) {
	if doc != nil {
		t.out.moveTo(doc.Pos())
	}
	t.out.write(t.docs.block(doc, command, extra))
}

/**
 * qualify returns the C++ name of a member of the package's namespace,
 * given the names within it (e.g. a type, then a method).
 *
 * @param names The names within the namespace.
 * @return The C++ name.
 */
func (t *translator) qualify(names ...string) string {
	qualified := t.pkg.namespace
	for _, name := range names {
		qualified += "::" + cppName(name)
	}
	return qualified
}

/**
 * typeSpec translates a named type.
 *
 * @param s The type's declaration.
 * @param doc Its comment, or nil.
 */
func (t *translator) typeSpec(s *ast.TypeSpec, doc *ast.CommentGroup) {
	name := cppName(s.Name.Name)
	template := templatePrefix(s.TypeParams)
	refs := t.index.crossReferences(t.qualify(s.Name.Name))
	if s.Assign.IsValid() {
		t.writeDoc(doc, "", refs)
		t.out.moveTo(s.Pos())
		t.out.write(template + "using " + name + " = " + typeText(s.Type) + ";")
		return
	}
	switch typ := s.Type.(type) {
	case *ast.StructType:
		t.writeDoc(doc, "", refs)
		t.out.moveTo(s.Pos())
		t.structType(template, s.Name.Name, typ)
	case *ast.InterfaceType:
		t.writeDoc(doc, "", refs)
		t.out.moveTo(s.Pos())
		t.interfaceType(template, s.Name.Name, typ)
	default:
		methods := t.pkg.methods[s.Name.Name]
		if len(methods) == 0 {
			t.writeDoc(doc, "", refs)
			t.out.moveTo(s.Pos())
			t.out.write(template + "using " + name + " = " + typeText(s.Type) + ";")
			return
		}
		// A class hides what the type really is, so the docs say.
		note := "<p>Go type: <tt>" + escapeDoc("type "+s.Name.Name+" "+goSource(t.pkg.fset, s.Type)) + "</tt></p>"
		t.writeDoc(doc, "", joinNonEmpty(note, refs))
		t.out.moveTo(s.Pos())
		header := template + "class " + name
		if bases := t.interfaceBases(s.Name.Name); len(bases) > 0 {
			header += " : " + strings.Join(bases, ", ")
		}
		t.out.write(header + " {" + t.methodDeclarations(s.Name.Name, false) + " };")
	}
}

/**
 * structType writes the struct type called name (its Go name), its fields
 * on their own lines, then its methods' declarations on the line of its
 * closing brace.
 *
 * @param template The template prefix for a generic type, or "".
 * @param name The type's Go name.
 * @param s The struct.
 */
func (t *translator) structType(template, name string, s *ast.StructType) {
	var bases []string
	for _, f := range s.Fields.List {
		if len(f.Names) == 0 {
			bases = append(bases, "public "+typeText(stripPointer(f.Type)))
		}
	}
	bases = append(bases, t.interfaceBases(name)...)
	header := template + "struct " + cppName(name)
	if len(bases) > 0 {
		header += " : " + strings.Join(bases, ", ")
	}
	t.out.write(header + " {")
	access := newAccess(true)
	for _, f := range s.Fields.List {
		if len(f.Names) == 0 {
			continue
		}
		refs := t.index.crossReferences(t.qualify(name, f.Names[0].Name))
		if f.Doc != nil || f.Comment == nil {
			t.writeDoc(f.Doc, "", refs)
		}
		t.out.moveTo(f.Pos())
		for _, n := range f.Names {
			t.out.write(access.switchTo(ast.IsExported(n.Name)) + typeText(f.Type) + " " + cppName(n.Name) + ";")
		}
		if f.Doc == nil && f.Comment != nil {
			t.out.write(" " + t.docs.trailing(f.Comment, refs))
		}
	}
	t.out.moveTo(s.Fields.Closing)
	t.out.write(t.methodDeclarations(name, true) + " };")
}

/**
 * interfaceType writes the interface type called name (its Go name) as a
 * class of pure virtual methods, each with its documentation; embedded
 * interfaces become base classes.
 *
 * @param template The template prefix for a generic type, or "".
 * @param name The type's Go name.
 * @param s The interface.
 */
func (t *translator) interfaceType(template, name string, s *ast.InterfaceType) {
	var bases []string
	for _, m := range s.Methods.List {
		if len(m.Names) == 0 {
			bases = append(bases, "public "+typeText(m.Type))
		}
	}
	header := template + "class " + cppName(name)
	if len(bases) > 0 {
		header += " : " + strings.Join(bases, ", ")
	}
	t.out.write(header + " { public:")
	for _, m := range s.Methods.List {
		fn, ok := m.Type.(*ast.FuncType)
		if !ok || len(m.Names) == 0 {
			continue
		}
		t.writeDoc(m.Doc, "", t.index.crossReferences(t.qualify(name, m.Names[0].Name)))
		t.out.moveTo(m.Pos())
		t.out.write("virtual " + resultText(fn.Results) + " " + cppName(m.Names[0].Name) +
			"(" + paramsText(fn.Params) + ") = 0;")
	}
	t.out.moveTo(s.Methods.Closing)
	t.out.write(" };")
}

/**
 * interfaceBases returns, as public base classes, the module's interfaces
 * the type called name implements, named relative to this package.
 *
 * @param name The type's Go name.
 * @return The base class specifiers.
 */
func (t *translator) interfaceBases(name string) []string {
	sym := t.index.lookup(t.qualify(name))
	if sym == nil {
		return nil
	}
	bases := make([]string, len(sym.Implements))
	for i, iface := range sym.Implements {
		bases[i] = "public " + strings.TrimPrefix(iface, t.pkg.namespace+"::")
	}
	return bases
}

/**
 * methodDeclarations returns, on one line, the declarations of every method
 * of the type called name, so their out-of-line definitions (written where
 * each method is) have something to attach to.
 *
 * @param name The type's Go name.
 * @param defaultPublic Whether the class starts out public (a struct) or
 * private.
 * @return The declarations.
 */
func (t *translator) methodDeclarations(name string, defaultPublic bool) string {
	access := newAccess(defaultPublic)
	var b strings.Builder
	for _, fn := range t.pkg.methods[name] {
		b.WriteString(" " + access.switchTo(fn.Name.IsExported()) + functionSignature(fn.Type, cppName(fn.Name.Name)) + ";")
	}
	return b.String()
}

/**
 * valueSpec translates constants or variables declared together.
 *
 * @param tok token.CONST or token.VAR.
 * @param s The declaration.
 * @param doc Its comment, or nil.
 */
func (t *translator) valueSpec(tok token.Token, s *ast.ValueSpec, doc *ast.CommentGroup) {
	refs := t.index.crossReferences(t.qualify(s.Names[0].Name))
	if doc != nil || s.Comment == nil {
		t.writeDoc(doc, "", refs)
	}
	t.out.moveTo(s.Pos())
	typ := "auto"
	if s.Type != nil {
		typ = typeText(s.Type)
	}
	prefix := ""
	if tok == token.CONST {
		prefix = "const "
	}
	for i, n := range s.Names {
		value := ""
		if i < len(s.Values) {
			if lit, ok := s.Values[i].(*ast.BasicLit); ok && !strings.HasPrefix(lit.Value, "`") {
				value = " = " + lit.Value
			}
		}
		t.out.write(prefix + typ + " " + cppName(n.Name) + value + ";")
	}
	if doc == nil && s.Comment != nil {
		t.out.write(" " + t.docs.trailing(s.Comment, refs))
	}
}

/**
 * funcDecl translates a function, or a method as an out-of-line definition
 * of the declaration its type's class carries.
 *
 * @param fn The function.
 */
func (t *translator) funcDecl(fn *ast.FuncDecl) {
	if fn.Recv == nil {
		t.writeDoc(fn.Doc, "", t.index.crossReferences(t.qualify(fn.Name.Name)))
	} else if source := t.implementedMethod(fn); source != "" {
		// The interface documents it; Doxygen shows the link back too.
		t.out.moveTo(fn.Doc.Pos())
		t.out.write(t.docs.block(nil, "@copydoc "+source,
			t.index.crossReferences(t.qualify(receiverName(fn), fn.Name.Name))))
	} else {
		t.writeDoc(fn.Doc, "", t.index.crossReferences(t.qualify(receiverName(fn), fn.Name.Name)))
	}
	t.out.moveTo(fn.Pos())
	if fn.Recv == nil {
		t.out.write(templatePrefix(fn.Type.TypeParams) + functionSignature(fn.Type, cppName(fn.Name.Name)))
		t.body(fn)
		return
	}
	args := typeParamArgs(fn)
	template := ""
	if args != "" {
		template = "template <typename " + strings.Join(strings.Split(strings.Trim(args, "<>"), ", "), ", typename ") + "> "
	}
	qualified := cppName(receiverName(fn)) + args + "::" + cppName(fn.Name.Name)
	t.out.write(template + functionSignature(fn.Type, qualified))
	t.body(fn)
}

/**
 * body writes an empty body spanning the lines of fn's real one.
 *
 * @param fn The function.
 */
func (t *translator) body(fn *ast.FuncDecl) {
	if fn.Body == nil {
		t.out.write(";")
		return
	}
	t.out.moveTo(fn.Body.Lbrace)
	t.out.write(" {")
	t.out.moveTo(fn.Body.Rbrace)
	t.out.write("}")
}

/**
 * implementsDoc matches a method comment that only says which interface
 * the method implements, e.g. "Name implements Command." or "Read
 * implements io.Reader."
 */
var implementsDoc = regexp.MustCompile(`^(\w+) implements ([\w.]+?)\.?$`)

/**
 * implementedMethod returns the C++ name of the interface method fn's
 * comment says it implements, if the comment says nothing else and the
 * interface is one of the module's that fn's type implements; else "".
 *
 * @param fn The method.
 * @return The interface method's C++ name, or "".
 */
func (t *translator) implementedMethod(fn *ast.FuncDecl) string {
	match := implementsDoc.FindStringSubmatch(strings.TrimSpace(commentText(fn.Doc)))
	if match == nil || match[1] != fn.Name.Name {
		return ""
	}
	iface := match[2][strings.LastIndex(match[2], ".")+1:]
	for _, candidate := range t.index.lookup(t.qualify(receiverName(fn))).implementations() {
		method := candidate + "::" + cppName(fn.Name.Name)
		if strings.HasSuffix(candidate, "::"+cppName(iface)) && t.index.lookup(method) != nil {
			return method
		}
	}
	return ""
}

/**
 * functionSignature spells "results name(params)".
 *
 * @param fn The function type.
 * @param name The name to declare.
 * @return The signature.
 */
func functionSignature(fn *ast.FuncType, name string) string {
	return resultText(fn.Results) + " " + name + "(" + paramsText(fn.Params) + ")"
}

/**
 * goSource prints a node as Go source on one line.
 *
 * @param fset The file set the node's positions belong to.
 * @param node The node.
 * @return The source, or "?" if it can't be printed.
 */
func goSource(fset *token.FileSet, node ast.Node) string {
	var b bytes.Buffer
	if err := printer.Fprint(&b, fset, node); err != nil {
		return "?"
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

/**
 * access tracks the current C++ access level while writing a class body.
 */
type access struct {
	/** public is true while members are public. */
	public bool
}

/**
 * newAccess starts in the class kind's default access: public for a
 * struct, private for a class.
 *
 * @param public Whether the class starts out public.
 * @return The tracker.
 */
func newAccess(public bool) *access {
	return &access{public: public}
}

/**
 * switchTo returns the access specifier needed before a member that should
 * be public (or not), or "" if the current level already fits.
 *
 * @param public Whether the next member is public.
 * @return The specifier, or "".
 */
func (a *access) switchTo(public bool) string {
	if a.public == public {
		return ""
	}
	a.public = public
	if public {
		return "public: "
	}
	return "private: "
}
