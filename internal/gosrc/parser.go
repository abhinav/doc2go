package gosrc

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"log"
	"maps"
	"slices"

	"braces.dev/errtrace"
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
	//
	// Includes unexported declarations.
	TopLevelDecls []string

	// Type information for the package.
	Info *types.Info
}

// Parser loads the contents of a package by parsing it from source.
type Parser struct {
	Logger *log.Logger
}

// ParsePackage parses all files in the package at the given path
// and fills a Package object with the result.
func (p *Parser) ParsePackage(ref *PackageRef) (*Package, error) {
	logger := p.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	fset := token.NewFileSet()
	files := make(map[string]*ast.File)
	syntax, err := parseFiles(fset, ref.Files, files)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	info := types.Info{
		Uses: make(map[*ast.Ident]types.Object),
		Defs: make(map[*ast.Ident]types.Object),
	}
	typesPkg, _ := (&types.Config{
		IgnoreFuncBodies: true,
		FakeImportC:      true,
		Importer:         newPackageImporter(ref.Imports),
		Error: func(error) {
			// Errors are expected.
			// We are using a fake importer.
		},
		DisableUnusedImportCheck: true,
	}).Check(ref.ImportPath, fset, syntax, &info)

	topLevelNames := make(map[string]struct{})
	pkgScope := typesPkg.Scope()
	for _, name := range pkgScope.Names() {
		obj := pkgScope.Lookup(name)
		switch obj.(type) {
		case *types.TypeName, *types.Const, *types.Var:
			topLevelNames[name] = struct{}{}

		case *types.Func:
			if name == "init" {
				// init functions are not top-level declarations.
				continue
			}

			typ := obj.Type().(*types.Signature)
			if typ.Recv() != nil {
				// Methods are not top-level declarations.
				continue
			}

			topLevelNames[name] = struct{}{}

		default:
			logger.Printf("unexpected object in package scope: (%T) %v", obj, obj)
		}
	}
	topLevel := slices.Sorted(maps.Keys(topLevelNames))

	testSyntax, err := parseFiles(fset, ref.TestFiles, nil /* fmap */)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	return &Package{
		Name:          ref.Name,
		ImportPath:    ref.ImportPath,
		Syntax:        syntax,
		TestSyntax:    testSyntax,
		Fset:          fset,
		TopLevelDecls: topLevel,
		Info:          &info,
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
			return nil, errtrace.Wrap(fmt.Errorf("parse file %q: %w", file, err))
		}
		if fmap != nil {
			fmap[file] = syntax[i]
		}
	}

	return syntax, nil
}

type packageImporter struct {
	pkgNames map[string]string // import path -> package name
}

func newPackageImporter(imports []ImportedPackage) *packageImporter {
	packageNames := make(map[string]string)
	for _, imp := range imports {
		packageNames[imp.ImportPath] = imp.Name
	}
	return &packageImporter{
		pkgNames: packageNames,
	}
}

func (p *packageImporter) Import(path string) (*types.Package, error) {
	panic("Import not expected to be called: use ImportFrom")
}

func (p *packageImporter) ImportFrom(path string, _ string, _ types.ImportMode) (*types.Package, error) {
	name, ok := p.pkgNames[path]
	if !ok {
		return nil, fmt.Errorf("unexpected package import: %q", path)
	}
	pkg := types.NewPackage(path, name)
	// important: package must be marked complete to be listed in imports
	pkg.MarkComplete()
	return pkg, nil
}
