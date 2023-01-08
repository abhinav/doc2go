package gosrc

import (
	"go/ast"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackage_simple(t *testing.T) {
	t.Parallel()

	srcFile := filepath.Join("testdata", "simple.go")
	testFile := filepath.Join("testdata", "simple_test.go")
	got, err := new(Parser).ParsePackage(&PackageRef{
		Name:       "foo",
		ImportPath: "example.com/foo",
		Files:      []string{srcFile},
		TestFiles:  []string{testFile},
	})
	require.NoError(t, err)

	assert.Equal(t, "foo", got.Name)
	assert.Equal(t, "example.com/foo", got.ImportPath)
	if files := got.Syntax; assert.Len(t, files, 1) {
		assert.Equal(t, srcFile, got.Fset.File(files[0].Pos()).Name())
	}
	// https://github.com/abhinav/doc2go/issues/15
	// if files := got.TestSyntax; assert.Len(t, files, 1) {
	// 	assert.Equal(t, testFile, got.Fset.File(files[0].Pos()).Name())
	// }
	assert.Equal(t, []string{
		"Constant",
		"Function",
		"Interface",
		"Struct",
		"Variable",
		"unexportedStruct",
	}, got.TopLevelDecls)
}

func TestParsePackage_namedImport(t *testing.T) {
	t.Parallel()

	srcFile := filepath.Join("testdata", "package_importer.go")
	got, err := new(Parser).ParsePackage(&PackageRef{
		Name:       "foo",
		ImportPath: "example.com/foo",
		Files:      []string{srcFile},
		Imports: []ImportedPackage{
			{
				Name:       "service",
				ImportPath: "example.com/service-go",
			},
		},
	})
	require.NoError(t, err)

	require.Len(t, got.Syntax, 1)

	var handlerType ast.Expr
	for _, decl := range got.Syntax[0].Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range genDecl.Specs {
			typ, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if typ.Name.Name != "Handler" {
				continue
			}

			handlerType = typ.Type
			break
		}

		if handlerType != nil {
			break
		}
	}

	require.NotNil(t, handlerType, "Handler type not found")
	st, ok := handlerType.(*ast.StructType)
	require.True(t, ok, "expected StructType, got %T", handlerType)
	require.NotEmpty(t, st.Fields.List)

	f := st.Fields.List[0]
	sel, ok := f.Type.(*ast.SelectorExpr)
	require.True(t, ok, "expected SelectorExpr, got %T", f.Type)

	x, ok := sel.X.(*ast.Ident)
	require.True(t, ok, "expected Ident, got %T", sel.X)

	require.NotNil(t, x.Obj)
	assert.Equal(t, "service", x.Obj.Name)
	assert.Equal(t, ast.Pkg, x.Obj.Kind)
}

func TestPackageRefImporter_notFound(t *testing.T) {
	t.Parallel()

	importer := packageRefImporter(&PackageRef{})
	imports := make(map[string]*ast.Object)

	_, err := importer(imports, "example.com/foo")
	assert.ErrorContains(t, err, `package "example.com/foo" not found`)
}
