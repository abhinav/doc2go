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
type Parser struct {
	// Reference to go/parser.ParseFile.
	//
	// May be overridden during tests.
	parseFile func(*token.FileSet, string, any, parser.Mode) (*ast.File, error)
}

// ParsePackage parses all files in the package at the given path
// and fills a Package object with the result.
func (p *Parser) ParsePackage(ref *PackageRef) (*Package, error) {
	parseFile := parser.ParseFile
	if p.parseFile != nil {
		parseFile = p.parseFile
	}

	fset := token.NewFileSet()

	syntax := make([]*ast.File, len(ref.Files))
	files := make(map[string]*ast.File)
	for i, file := range ref.Files {
		var err error
		syntax[i], err = parseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse file %q: %w", file, err)
		}
		files[file] = syntax[i]
	}

	var topLevel []string
	if pkg, _ := ast.NewPackage(fset, files, nil, nil); pkg != nil {
		topLevel = make([]string, 0, len(pkg.Scope.Objects))
		for name := range pkg.Scope.Objects {
			topLevel = append(topLevel, name)
		}
		sort.Strings(topLevel)
	}

	testSyntax := make([]*ast.File, len(ref.TestFiles))
	for i, file := range ref.TestFiles {
		var err error
		testSyntax[i], err = parseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse file %q: %w", file, err)
		}
	}

	return &Package{
		Name:          ref.Name,
		ImportPath:    ref.ImportPath,
		Syntax:        syntax,
		TestSyntax:    testSyntax,
		Fset:          fset,
		TopLevelDecls: topLevel,
	}, nil
}
