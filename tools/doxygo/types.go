package main

import (
	"go/ast"
	"strings"
)

/**
 * cppKeywords are C++ keywords that are valid Go identifiers, including
 * C++20's contextual "module" and "import", which Doxygen treats as
 * keywords; such names get a trailing underscore so Doxygen's parser
 * doesn't trip over them.
 */
var cppKeywords = map[string]bool{
	"auto": true, "class": true, "delete": true, "explicit": true, "friend": true,
	"inline": true, "namespace": true, "new": true, "operator": true, "private": true,
	"protected": true, "public": true, "register": true, "template": true, "this": true,
	"throw": true, "typename": true, "union": true, "using": true, "virtual": true,
	"volatile": true, "signed": true, "unsigned": true, "static": true, "extern": true,
	"char": true, "double": true, "float": true, "long": true, "short": true, "void": true,
	"int": true, "bool": true, "typedef": true, "enum": true, "while": true, "do": true,
	"catch": true, "try": true, "mutable": true, "export": true, "and": true, "or": true,
	"not": true, "xor": true, "requires": true, "concept": true, "module": true, "import": true,
}

/**
 * cppName returns a Go identifier as a usable C++ name.
 *
 * @param name The Go identifier.
 * @return The name, with "_" appended if it is a C++ keyword.
 */
func cppName(name string) string {
	if cppKeywords[name] {
		return name + "_"
	}
	return name
}

/**
 * goBuiltinTypes are Go's predeclared types whose names are also C++
 * keywords; used as types, they keep their names.
 */
var goBuiltinTypes = map[string]bool{"int": true, "bool": true}

/**
 * typeName returns the C++ name for a type named by a Go identifier: a
 * predeclared type keeps its name, anything else goes through cppName.
 *
 * @param name The Go identifier.
 * @return The C++ name.
 */
func typeName(name string) string {
	if goBuiltinTypes[name] {
		return name
	}
	return cppName(name)
}

/**
 * typeText spells a Go type expression the C++ way (see the package
 * comment), so Doxygen can parse it and link the names in it.
 *
 * @param expr The type expression.
 * @return The C++ spelling.
 */
func typeText(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return typeName(t.Name)
	case *ast.SelectorExpr:
		return typeText(t.X) + "::" + cppName(t.Sel.Name)
	case *ast.StarExpr:
		return typeText(t.X) + "*"
	case *ast.ParenExpr:
		return typeText(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "slice<" + typeText(t.Elt) + ">"
		}
		return "array<" + typeText(t.Elt) + ", " + exprText(t.Len) + ">"
	case *ast.Ellipsis:
		return typeText(t.Elt) + "..."
	case *ast.MapType:
		return "map<" + typeText(t.Key) + ", " + typeText(t.Value) + ">"
	case *ast.ChanType:
		return "chan<" + typeText(t.Value) + ">"
	case *ast.FuncType:
		return "func<" + resultText(t.Results) + "(" + strings.Join(fieldTypes(t.Params), ", ") + ")>"
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "any"
		}
		return "interface"
	case *ast.StructType:
		return "struct"
	case *ast.IndexExpr:
		return typeText(t.X) + "<" + typeText(t.Index) + ">"
	case *ast.IndexListExpr:
		args := make([]string, len(t.Indices))
		for i, index := range t.Indices {
			args[i] = typeText(index)
		}
		return typeText(t.X) + "<" + strings.Join(args, ", ") + ">"
	}
	return "auto"
}

/**
 * exprText returns simple expressions (literals, names) as written, and
 * "..." for anything more complex.
 *
 * @param expr The expression.
 * @return Its text.
 */
func exprText(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Value
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprText(e.X) + "::" + e.Sel.Name
	}
	return "..."
}

/**
 * resultText spells a function's results as one C++ return type: void for
 * none, the type for one, tuple<...> for several.
 *
 * @param results The result list, or nil.
 * @return The return type.
 */
func resultText(results *ast.FieldList) string {
	types := fieldTypes(results)
	switch len(types) {
	case 0:
		return "void"
	case 1:
		return types[0]
	}
	return "tuple<" + strings.Join(types, ", ") + ">"
}

/**
 * fieldTypes returns one type per declared name in fields (a parameter or
 * result list), so "a, b int" gives two.
 *
 * @param fields The field list, or nil.
 * @return The types.
 */
func fieldTypes(fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}
	var types []string
	for _, f := range fields.List {
		for range max(1, len(f.Names)) {
			types = append(types, typeText(f.Type))
		}
	}
	return types
}

/**
 * paramsText spells a parameter list as C++ parameters, "type name, ...".
 *
 * @param params The parameter list, or nil.
 * @return The C++ parameters.
 */
func paramsText(params *ast.FieldList) string {
	if params == nil {
		return ""
	}
	var parts []string
	for _, f := range params.List {
		if len(f.Names) == 0 {
			parts = append(parts, typeText(f.Type))
		}
		for _, name := range f.Names {
			if name.Name == "_" {
				parts = append(parts, typeText(f.Type))
			} else {
				parts = append(parts, typeText(f.Type)+" "+cppName(name.Name))
			}
		}
	}
	return strings.Join(parts, ", ")
}

/**
 * templatePrefix returns "template <typename A, typename B> " for a list of
 * type parameters, or "" for none.
 *
 * @param params The type parameters, or nil.
 * @return The prefix.
 */
func templatePrefix(params *ast.FieldList) string {
	if params == nil || len(params.List) == 0 {
		return ""
	}
	var names []string
	for _, f := range params.List {
		for _, name := range f.Names {
			names = append(names, "typename "+name.Name)
		}
	}
	return "template <" + strings.Join(names, ", ") + "> "
}

/**
 * typeParamArgs returns "<A, B>" naming a receiver's type parameters, or "".
 *
 * @param fn The method.
 * @return The arguments.
 */
func typeParamArgs(fn *ast.FuncDecl) string {
	switch t := stripPointer(fn.Recv.List[0].Type).(type) {
	case *ast.IndexExpr:
		return "<" + typeText(t.Index) + ">"
	case *ast.IndexListExpr:
		args := make([]string, len(t.Indices))
		for i, index := range t.Indices {
			args[i] = typeText(index)
		}
		return "<" + strings.Join(args, ", ") + ">"
	}
	return ""
}

/**
 * stripPointer removes a leading * from a type expression.
 *
 * @param expr The type expression.
 * @return The pointed-to type, or expr.
 */
func stripPointer(expr ast.Expr) ast.Expr {
	if star, ok := expr.(*ast.StarExpr); ok {
		return star.X
	}
	return expr
}
