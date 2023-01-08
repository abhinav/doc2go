package gosrc

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// Package is a package that has been loaded from disk.
type Package struct {
	// Name of the package.
	Name string

	// Import path of the package.
	ImportPath string

	// Parsed ASTs of all source files in the package.
	Syntax []*ast.File

	// Parsed ASTs of all test files in the package.
	TestSyntax []*ast.File

	// FileSet used to parse these files.
	Fset *token.FileSet

	// Names of top-level declarations defined in this package.
	TopLevelDecls []string
}

// Parser loads the contents of a package by parsing it from source.
type Parser struct{}

// ParsePackage parses all files in the package at the given path
// and fills a Package object with the result.
func (*Parser) ParsePackage(ref *PackageRef) (*Package, error) {
	fset := token.NewFileSet()

	files := make(map[string]*ast.File)
	syntax, err := parseFiles(fset, ref.Files, files)
	if err != nil {
		return nil, err
	}

	var topLevel []string
	if pkg, _ := ast.NewPackage(fset, files, packageRefImporter(ref), nil); pkg != nil {
		topLevel = make([]string, 0, len(pkg.Scope.Objects))
		for name := range pkg.Scope.Objects {
			topLevel = append(topLevel, name)
		}
		sort.Strings(topLevel)
	}

	// TODO:
	// Parse test files to extract example tests.
	// https://github.com/abhinav/doc2go/issues/15
	//
	// testSyntax, err := parseFiles(fset, ref.TestFiles, nil)
	// if err != nil {
	// 	return nil, err
	// }

	return &Package{
		Name:       ref.Name,
		ImportPath: ref.ImportPath,
		Syntax:     syntax,
		// TestSyntax:    testSyntax,
		Fset:          fset,
		TopLevelDecls: topLevel,
	}, nil
}

// parseFiles parses the given list of files,
// and returns ASTs for them in the same order.
// If fmap is non-nil, this will also populate the map with entries
// for the parsed files.
func parseFiles(fset *token.FileSet, files []string, fmap map[string]*ast.File) ([]*ast.File, error) {
	if len(files) == 0 {
		return nil, nil
	}

	syntax := make([]*ast.File, len(files))
	for i, file := range files {
		var err error
		syntax[i], err = parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse file %q: %w", file, err)
		}
		if fmap != nil {
			fmap[file] = syntax[i]
		}
	}

	return syntax, nil
}

func packageRefImporter(ref *PackageRef) ast.Importer {
	packageNames := make(map[string]string, len(ref.Imports)) // import path => name
	for _, imp := range ref.Imports {
		packageNames[imp.ImportPath] = imp.Name
	}

	return func(imports map[string]*ast.Object, path string) (pkg *ast.Object, err error) {
		if pkg := imports[path]; pkg != nil {
			return pkg, nil
		}

		name, ok := packageNames[path]
		if !ok {
			return nil, fmt.Errorf("package %q not found", path)
		}

		pkg = ast.NewObj(ast.Pkg, name)
		pkg.Data = ast.NewScope(nil)
		imports[path] = pkg
		return pkg, nil
	}
}
