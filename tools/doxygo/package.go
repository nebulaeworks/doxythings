package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
)

/**
 * goPackage is the parsed package a filtered file belongs to. Methods can be
 * declared in any file of a package, but C++ needs them declared inside
 * their class, so every file of the package is read.
 */
type goPackage struct {
	/** fset maps positions in every file to lines. */
	fset *token.FileSet
	/** module is the module the package belongs to. */
	module *module
	/** path is the package's import path. */
	path string
	/**
	 * namespace is the C++ namespace the package becomes, e.g.
	 * "doxythings::internal::render".
	 */
	namespace string
	/**
	 * methods maps each receiver type name to its methods, in source order
	 * of their files.
	 */
	methods map[string][]*ast.FuncDecl
}

/**
 * loadPackage parses every non-test Go file in the directory of path.
 *
 * @param path The Go file being filtered.
 * @return The package, or an error if a file can't be read or parsed.
 */
func loadPackage(path string) (*goPackage, error) {
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	m, err := findModule(dir)
	if err != nil {
		return nil, err
	}
	names, err := sourceFiles(dir)
	if err != nil {
		return nil, err
	}
	pkg := &goPackage{
		fset:    token.NewFileSet(),
		module:  m,
		path:    m.importPath(dir),
		methods: map[string][]*ast.FuncDecl{},
	}
	pkg.namespace = m.namespace(pkg.path)
	for _, name := range names {
		file, err := parser.ParseFile(pkg.fset, name, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
				receiver := receiverName(fn)
				pkg.methods[receiver] = append(pkg.methods[receiver], fn)
			}
		}
	}
	return pkg, nil
}

/**
 * receiverName returns the name of a method's receiver type, without any
 * pointer or type parameters.
 *
 * @param fn The method.
 * @return The receiver type's name, or "".
 */
func receiverName(fn *ast.FuncDecl) string {
	expr := fn.Recv.List[0].Type
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
		case *ast.IndexExpr:
			expr = t.X
		case *ast.IndexListExpr:
			expr = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
		}
	}
}

/**
 * docFile returns the file of the module package with the import path
 * that holds its package comment (else its first file), relative to the
 * module root with forward slashes, or "" if the package has no files.
 * It is the file an import of the package is shown as including.
 *
 * @param importPath The package's import path.
 * @return The file's path, or "".
 */
func (m *module) docFile(importPath string) string {
	files, err := sourceFiles(m.dir(importPath))
	if err != nil || len(files) == 0 {
		return ""
	}
	chosen := files[0]
	for _, name := range files {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.PackageClauseOnly|parser.ParseComments)
		if err == nil && file.Doc != nil {
			chosen = name
			break
		}
	}
	rel, err := filepath.Rel(m.root, chosen)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}
