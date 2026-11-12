package main

import (
	"errors"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

/**
 * module is a Go module: the directory holding its go.mod and the module
 * path that file declares. It maps package directories to the C++
 * namespaces doxygo puts them in.
 */
type module struct {
	/** root is the absolute directory holding go.mod. */
	root string
	/** path is the module path, e.g. "github.com/nebulaeworks/doxythings". */
	path string
}

/**
 * findModule returns the module containing dir, or a module rooted at dir
 * with dir's name as its path if dir isn't inside one.
 *
 * @param dir The directory.
 * @return The module, or an error if dir can't be made absolute.
 */
func findModule(dir string) (*module, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for root := dir; ; {
		if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
			return &module{root: root, path: modulePath(string(data))}, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return &module{root: dir, path: filepath.Base(dir)}, nil
		}
		root = parent
	}
}

/**
 * modulePath returns the module path declared in go.mod's content.
 *
 * @param gomod The content of go.mod.
 * @return The module path, or "module" if there is none.
 */
func modulePath(gomod string) string {
	for _, line := range strings.Split(gomod, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return "module"
}

/**
 * importPath returns the import path of the package in dir, which must be
 * inside the module.
 *
 * @param dir The package's directory.
 * @return Its import path.
 */
func (m *module) importPath(dir string) string {
	rel, err := filepath.Rel(m.root, dir)
	if err != nil || rel == "." {
		return m.path
	}
	return m.path + "/" + filepath.ToSlash(rel)
}

/**
 * contains reports whether the import path names a package of the module.
 *
 * @param importPath The import path.
 * @return Whether it is in the module.
 */
func (m *module) contains(importPath string) bool {
	return importPath == m.path || strings.HasPrefix(importPath, m.path+"/")
}

/**
 * dir returns the directory of the module package with the import path.
 *
 * @param importPath The package's import path.
 * @return Its directory.
 */
func (m *module) dir(importPath string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(importPath, m.path), "/")
	return filepath.Join(m.root, filepath.FromSlash(rel))
}

/**
 * namespace returns the C++ namespace for the module package with the
 * import path: the module path's last element, then the package's
 * directories within the module, e.g. "doxythings::internal::render".
 *
 * @param importPath The package's import path.
 * @return The namespace.
 */
func (m *module) namespace(importPath string) string {
	parts := []string{cppName(m.path[strings.LastIndex(m.path, "/")+1:])}
	if rel := strings.TrimPrefix(strings.TrimPrefix(importPath, m.path), "/"); rel != "" {
		for _, part := range strings.Split(rel, "/") {
			parts = append(parts, cppName(part))
		}
	}
	return strings.Join(parts, "::")
}

/**
 * sourceFiles lists the Go files of the package in dir that a build on this
 * machine would compile, excluding tests, sorted by name.
 *
 * @param dir The package's directory.
 * @return The files' paths, or an error if dir can't be read.
 */
func sourceFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if ok, err := build.Default.MatchFile(dir, name); err == nil && ok {
			names = append(names, filepath.Join(dir, name))
		}
	}
	sort.Strings(names)
	return names, nil
}

/**
 * packageDirs lists the module's package directories: every directory
 * below the root with Go files, except hidden ones, testdata, vendor and
 * nested modules. extra, if not empty, is added even if it would be
 * skipped, so a file under testdata can still be filtered.
 *
 * @param extra A directory to include anyway, or "".
 * @return The directories, or an error if the module can't be walked.
 */
func (m *module) packageDirs(extra string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(m.root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if path != m.root {
			name := d.Name()
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			}
		}
		if files, err := sourceFiles(path); err == nil && len(files) > 0 {
			dirs = append(dirs, path)
		}
		return nil
	})
	if extra != "" && !containsString(dirs, extra) {
		dirs = append(dirs, extra)
	}
	return dirs, err
}

/**
 * containsString reports whether list holds s.
 *
 * @param list The list.
 * @param s The string.
 * @return Whether list holds s.
 */
func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

/**
 * typedPackage is one package of the module, parsed and type-checked.
 */
type typedPackage struct {
	/** path is the package's import path. */
	path string
	/** files are its parsed source files. */
	files []*ast.File
	/**
	 * info holds what the type checker resolved, notably which object
	 * each identifier uses.
	 */
	info *types.Info
	/** types is the checked package. */
	types *types.Package
}

/**
 * typedModule is every package of a module, type-checked together so that
 * identifiers can be resolved across packages.
 */
type typedModule struct {
	/** module is the module the packages belong to. */
	*module
	/** fset maps positions in every parsed file to lines. */
	fset *token.FileSet
	/** packages maps import paths to the module's packages. */
	packages map[string]*typedPackage
	/** dirs maps import paths to the directories not yet checked. */
	dirs map[string]string
	/** external imports packages from outside the module. */
	external types.Importer
}

/**
 * typeCheck parses and type-checks every package of m, plus the package in
 * extraDir. Type errors are ignored, so a package that doesn't compile is
 * still checked as far as it can be.
 *
 * @param m The module.
 * @param extraDir A package directory to check as well, or "".
 * @return The checked module, or an error if a file can't be read or parsed.
 */
func typeCheck(m *module, extraDir string) (*typedModule, error) {
	dirs, err := m.packageDirs(extraDir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	tm := &typedModule{
		module:   m,
		fset:     fset,
		packages: map[string]*typedPackage{},
		dirs:     map[string]string{},
		external: importer.ForCompiler(fset, "source", nil),
	}
	for _, dir := range dirs {
		tm.dirs[m.importPath(dir)] = dir
	}
	for _, dir := range dirs {
		if _, err := tm.Import(m.importPath(dir)); err != nil {
			return nil, err
		}
	}
	return tm, nil
}

/**
 * Import returns the package with the import path, checking it first if
 * it is one of the module's. It makes typedModule a types.Importer.
 *
 * @param path The import path.
 * @return The package, or an error if it can't be found or imports itself.
 */
func (tm *typedModule) Import(path string) (*types.Package, error) {
	if p, ok := tm.packages[path]; ok {
		if p.types == nil {
			return nil, errors.New("import cycle through " + path)
		}
		return p.types, nil
	}
	dir, ok := tm.dirs[path]
	if !ok {
		return tm.external.Import(path)
	}
	p := &typedPackage{path: path}
	tm.packages[path] = p
	files, err := sourceFiles(dir)
	if err != nil {
		return nil, err
	}
	for _, name := range files {
		file, err := parser.ParseFile(tm.fset, name, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		p.files = append(p.files, file)
	}
	p.info = &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}}
	config := types.Config{Importer: tm, Error: func(error) {}}
	p.types, _ = config.Check(path, tm.fset, p.files, p.info)
	return p.types, nil
}
