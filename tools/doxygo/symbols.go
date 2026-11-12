package main

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"
)

/**
 * symbolKind says what a symbol is, which decides how it is referred to.
 */
type symbolKind string

const (
	/**
	 * kindFunction is a function or method, shown with "()" and drawn in
	 * call graphs.
	 */
	kindFunction symbolKind = "function"
	/**
	 * kindVariable is a package-level variable or constant, or a struct
	 * field.
	 */
	kindVariable symbolKind = "variable"
	/** kindType is a named type: never a reference, but a doc link target. */
	kindType symbolKind = "type"
)

/**
 * symbol is one documented entity of the module, by its C++ name, with
 * what it refers to and what refers to it.
 */
type symbol struct {
	/** Kind says what the symbol is. */
	Kind symbolKind `json:"kind"`
	/** Namespace is the C++ namespace of the symbol's package. */
	Namespace string `json:"namespace"`
	/** Brief is the first sentence of its documentation, as plain text. */
	Brief string `json:"brief,omitempty"`
	/**
	 * Refs are the C++ names of the functions and variables it refers to,
	 * sorted.
	 */
	Refs []string `json:"refs,omitempty"`
	/** Callers are the C++ names of the symbols referring to it, sorted. */
	Callers []string `json:"callers,omitempty"`
	/**
	 * Implements are, for a type, the C++ names of the module's interfaces
	 * it (or a pointer to it) implements, sorted.
	 */
	Implements []string `json:"implements,omitempty"`
}

/**
 * symbolIndex is every symbol of a module by C++ name, e.g.
 * "doxythings::internal::render::FormatTime".
 */
type symbolIndex struct {
	/** Symbols maps C++ names to symbols. */
	Symbols map[string]*symbol `json:"symbols"`
}

/**
 * lookup returns the symbol called name, or nil.
 *
 * @param name The C++ name.
 * @return The symbol, or nil.
 */
func (x *symbolIndex) lookup(name string) *symbol {
	if x == nil {
		return nil
	}
	return x.Symbols[name]
}

/**
 * implementations returns the interfaces a type symbol implements; none
 * for a nil symbol.
 *
 * @return The interfaces' C++ names.
 */
func (s *symbol) implementations() []string {
	if s == nil {
		return nil
	}
	return s.Implements
}

/**
 * indexBuilder builds a symbolIndex from a type-checked module.
 */
type indexBuilder struct {
	/** module is the type-checked module. */
	module *typedModule
	/** index is the index being built. */
	index *symbolIndex
	/**
	 * fieldOwners maps each field of the module's named struct types to
	 * the C++ name of its struct.
	 */
	fieldOwners map[*types.Var]string
}

/**
 * buildIndex indexes every symbol of tm and the references between them.
 *
 * @param tm The type-checked module.
 * @return The index.
 */
func buildIndex(tm *typedModule) *symbolIndex {
	b := &indexBuilder{
		module:      tm,
		index:       &symbolIndex{Symbols: map[string]*symbol{}},
		fieldOwners: map[*types.Var]string{},
	}
	paths := make([]string, 0, len(tm.packages))
	for path := range tm.packages {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		b.collectFieldOwners(tm.packages[path])
	}
	for _, path := range paths {
		p := tm.packages[path]
		for _, file := range p.files {
			for _, decl := range file.Decls {
				b.declaration(p, decl)
			}
		}
	}
	b.linkCallers()
	b.linkImplementations(paths)
	return b.index
}

/**
 * linkImplementations fills in each type's Implements: Go types satisfy
 * interfaces implicitly, so the docs show it as inheritance.
 *
 * @param paths The module's import paths, sorted.
 */
func (b *indexBuilder) linkImplementations(paths []string) {
	type named struct {
		name string
		typ  types.Type
	}
	var interfaces, concrete []named
	for _, path := range paths {
		p := b.module.packages[path]
		if p.types == nil {
			continue
		}
		scope := p.types.Scope()
		for _, name := range scope.Names() {
			typeName, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || typeName.IsAlias() {
				continue
			}
			if n, ok := typeName.Type().(*types.Named); ok && n.TypeParams().Len() > 0 {
				continue // generic types implement nothing until instantiated
			}
			qualified := b.module.namespace(path) + "::" + cppName(name)
			if iface, ok := typeName.Type().Underlying().(*types.Interface); ok {
				if iface.NumMethods() > 0 {
					interfaces = append(interfaces, named{qualified, typeName.Type()})
				}
				continue
			}
			concrete = append(concrete, named{qualified, typeName.Type()})
		}
	}
	for _, c := range concrete {
		sym := b.index.Symbols[c.name]
		if sym == nil {
			continue
		}
		for _, i := range interfaces {
			iface := i.typ.Underlying().(*types.Interface)
			if hasMethods(c.typ, iface) || hasMethods(types.NewPointer(c.typ), iface) {
				sym.Implements = append(sym.Implements, i.name)
			}
		}
	}
}

/**
 * collectFieldOwners records which struct each field of p's named struct
 * types belongs to, since a field object doesn't know.
 *
 * @param p The package.
 */
func (b *indexBuilder) collectFieldOwners(p *typedPackage) {
	if p.types == nil {
		return
	}
	scope := p.types.Scope()
	for _, name := range scope.Names() {
		typeName, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		s, ok := typeName.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		owner := b.module.namespace(p.path) + "::" + cppName(name)
		for i := range s.NumFields() {
			if field := s.Field(i); !field.Embedded() {
				b.fieldOwners[field] = owner
			}
		}
	}
}

/**
 * declaration indexes the symbols one top-level declaration of p declares,
 * and what each refers to.
 *
 * @param p The package the declaration is in.
 * @param decl The declaration.
 */
func (b *indexBuilder) declaration(p *typedPackage, decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if name, ok := b.nameOf(p.info.Defs[d.Name]); ok {
			b.add(name, kindFunction, p, d.Doc, d.Body)
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				b.typeSpec(p, s, docOr(s.Doc, d))
			case *ast.ValueSpec:
				for _, ident := range s.Names {
					if name, ok := b.nameOf(p.info.Defs[ident]); ok {
						b.add(name, kindVariable, p, docOr(docOr(s.Doc, d), nil, s.Comment), valueNodes(s)...)
					}
				}
			}
		}
	}
}

/**
 * typeSpec indexes a named type and its fields or interface methods.
 *
 * @param p The package the type is in.
 * @param s The type's declaration.
 * @param doc Its comment, or nil.
 */
func (b *indexBuilder) typeSpec(p *typedPackage, s *ast.TypeSpec, doc *ast.CommentGroup) {
	name, ok := b.nameOf(p.info.Defs[s.Name])
	if !ok {
		return
	}
	b.add(name, kindType, p, doc)
	switch t := s.Type.(type) {
	case *ast.StructType:
		for _, field := range t.Fields.List {
			for _, ident := range field.Names {
				if fieldName, ok := b.nameOf(p.info.Defs[ident]); ok {
					b.add(fieldName, kindVariable, p, docOr(field.Doc, nil, field.Comment))
				}
			}
		}
	case *ast.InterfaceType:
		for _, method := range t.Methods.List {
			for _, ident := range method.Names {
				if methodName, ok := b.nameOf(p.info.Defs[ident]); ok {
					b.add(methodName, kindFunction, p, method.Doc)
				}
			}
		}
	}
}

/**
 * add indexes the symbol called name, with the functions and variables
 * the code in bodies refers to.
 *
 * @param name The symbol's C++ name.
 * @param kind What it is.
 * @param p The package it is in.
 * @param doc Its comment, or nil.
 * @param bodies The code whose references it makes, if any.
 */
func (b *indexBuilder) add(name string, kind symbolKind, p *typedPackage, doc *ast.CommentGroup, bodies ...ast.Node) {
	sym := &symbol{Kind: kind, Namespace: b.module.namespace(p.path), Brief: brief(doc)}
	refs := map[string]bool{}
	for _, body := range bodies {
		if body == nil {
			continue
		}
		ast.Inspect(body, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if target, ok := b.nameOf(p.info.Uses[ident]); ok && target != name {
				if _, isType := p.info.Uses[ident].(*types.TypeName); !isType {
					refs[target] = true
				}
			}
			return true
		})
	}
	sym.Refs = sortedKeys(refs)
	b.index.Symbols[name] = sym
}

/**
 * linkCallers fills in each symbol's Callers from the others' Refs.
 */
func (b *indexBuilder) linkCallers() {
	callers := map[string]map[string]bool{}
	for name, sym := range b.index.Symbols {
		for _, ref := range sym.Refs {
			if callers[ref] == nil {
				callers[ref] = map[string]bool{}
			}
			callers[ref][name] = true
		}
	}
	for name, set := range callers {
		if sym := b.index.Symbols[name]; sym != nil {
			sym.Callers = sortedKeys(set)
		}
	}
}

/**
 * nameOf returns the C++ name doxygo gives obj, if obj is a package-level
 * function, variable, constant or type of the module, a method, or a field
 * of a named struct.
 *
 * @param obj The object, or nil.
 * @return The C++ name, and whether obj has one.
 */
func (b *indexBuilder) nameOf(obj types.Object) (string, bool) {
	if obj == nil || obj.Pkg() == nil || !b.module.contains(obj.Pkg().Path()) {
		return "", false
	}
	namespace := b.module.namespace(obj.Pkg().Path())
	atPackageLevel := obj.Parent() == obj.Pkg().Scope()
	switch o := obj.(type) {
	case *types.Func:
		o = o.Origin()
		if recv := o.Type().(*types.Signature).Recv(); recv != nil {
			if typeName := receiverTypeName(recv.Type()); typeName != "" {
				return namespace + "::" + cppName(typeName) + "::" + cppName(o.Name()), true
			}
			return "", false
		}
		return namespace + "::" + cppName(o.Name()), atPackageLevel
	case *types.Var:
		o = o.Origin()
		if o.IsField() {
			owner, ok := b.fieldOwners[o]
			return owner + "::" + cppName(o.Name()), ok
		}
		return namespace + "::" + cppName(o.Name()), o.Parent() == o.Pkg().Scope()
	case *types.Const, *types.TypeName:
		return namespace + "::" + cppName(obj.Name()), atPackageLevel
	}
	return "", false
}

/**
 * hasMethods reports whether t has every method of iface, with identical
 * signatures. Unlike types.Implements, it doesn't accept types with errors
 * in them (such as an embedded type that doesn't exist) as having any
 * method.
 *
 * @param t The type.
 * @param iface The interface.
 * @return Whether t implements iface.
 */
func hasMethods(t types.Type, iface *types.Interface) bool {
	for i := range iface.NumMethods() {
		want := iface.Method(i)
		obj, _, _ := types.LookupFieldOrMethod(t, false, want.Pkg(), want.Name())
		got, ok := obj.(*types.Func)
		if !ok || !types.Identical(got.Type().(*types.Signature), want.Type().(*types.Signature)) {
			return false
		}
	}
	return true
}

/**
 * receiverTypeName returns the name of a method's receiver type (or, for
 * an interface method, of its interface), or "" if it has none.
 *
 * @param t The receiver's type.
 * @return The type's name, or "".
 */
func receiverTypeName(t types.Type) string {
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	if named, ok := t.(*types.Named); ok {
		return named.Origin().Obj().Name()
	}
	return ""
}

/**
 * docOr returns doc, else the doc of a declaration with a single spec,
 * else the first of trailing that isn't nil.
 *
 * @param doc The spec's own comment, or nil.
 * @param decl The declaration holding the spec, or nil.
 * @param trailing Comments after the spec, tried in order.
 * @return The comment, or nil.
 */
func docOr(doc *ast.CommentGroup, decl *ast.GenDecl, trailing ...*ast.CommentGroup) *ast.CommentGroup {
	if doc != nil {
		return doc
	}
	if decl != nil && len(decl.Specs) == 1 && decl.Doc != nil {
		return decl.Doc
	}
	for _, t := range trailing {
		if t != nil {
			return t
		}
	}
	return nil
}

/**
 * valueNodes returns the initial values of a const or var spec, as the
 * nodes whose references the declared names make.
 *
 * @param s The spec.
 * @return Its values.
 */
func valueNodes(s *ast.ValueSpec) []ast.Node {
	nodes := make([]ast.Node, len(s.Values))
	for i, v := range s.Values {
		nodes[i] = v
	}
	return nodes
}

/**
 * brief returns the first sentence of a doc comment as plain text: no
 * Doxygen commands, Markdown code marks, doc link brackets or line breaks.
 *
 * @param doc The comment, or nil.
 * @return The sentence.
 */
func brief(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	var words []string
	for _, word := range strings.Fields(commentText(doc)) {
		if strings.HasPrefix(word, "@") || strings.HasPrefix(word, `\`) {
			if len(words) > 0 {
				break // a command after the first sentence's start ends it
			}
			continue
		}
		words = append(words, strings.NewReplacer("`", "", "[", "", "]", "").Replace(word))
		if strings.HasSuffix(word, ".") {
			break
		}
	}
	return strings.Join(words, " ")
}

/**
 * sortedKeys returns a set's members in order.
 *
 * @param set The set.
 * @return Its members, sorted.
 */
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
