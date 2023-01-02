package gosrc

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeclFormatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string

		// One or more declarations.
		// Only the last of these will be formatted;
		// the others are auxilliary.
		give string

		imports map[string]string // import path => package name

		// List of top-level declarations.
		topLevel []string

		// Formatted form of the last declaration in 'give'.
		want string

		// Special regions of 'want'.
		regions []Region
	}{
		{
			desc:     "struct",
			give:     "type Foo struct{ Bar string }",
			topLevel: []string{"Foo"},
			want:     "type Foo struct{ Bar string }",
			regions: []Region{
				{
					Label: &DeclLabel{
						Parent: "Foo",
						Name:   "Bar",
					},
					Offset: 17,
					Length: 3,
				},
				{
					Label: &EntityRefLabel{
						ImportPath: Builtin,
						Name:       "string",
					},
					Offset: 21,
					Length: 6,
				},
			},
		},
		{
			desc: "interface",
			give: "type Bar struct{}\n" +
				"type Foo interface{ Bar(x Bar) }\n",
			topLevel: []string{"Foo", "Bar"},
			want:     "type Foo interface{ Bar(x Bar) }",
			regions: []Region{
				{
					Label: &DeclLabel{
						Parent: "Foo",
						Name:   "Bar",
					},
					Offset: 20,
					Length: 3,
				},
				{
					Label: &EntityRefLabel{
						Name: "Bar",
					},
					Offset: 26,
					Length: 3,
				},
			},
		},
		{
			desc:     "function",
			give:     "func Foo(s string) int",
			topLevel: []string{"Foo"},
			want:     "func Foo(s string) int",
			regions: []Region{
				{
					Label: &EntityRefLabel{
						ImportPath: Builtin,
						Name:       "string",
					},
					Offset: 11,
					Length: 6,
				},
				{
					Label: &EntityRefLabel{
						ImportPath: Builtin,
						Name:       "int",
					},
					Offset: 19,
					Length: 3,
				},
			},
		},
		{
			desc: "method",
			give: "type Foo struct{}\n" +
				"type Baz struct{}\n" +
				"func (*Foo) Bar() Baz\n",
			topLevel: []string{"Foo", "Baz"},
			want:     "func (*Foo) Bar() Baz",
			regions: []Region{
				{
					Label:  &EntityRefLabel{Name: "Foo"},
					Offset: 7,
					Length: 3,
				},
				{
					Label:  &EntityRefLabel{Name: "Baz"},
					Offset: 18,
					Length: 3,
				},
			},
		},
		{
			desc:     "value",
			give:     `var Foo = "foo"`,
			topLevel: []string{"Foo"},
			want:     `var Foo = "foo"`,
			regions: []Region{
				{
					Label:  &DeclLabel{Name: "Foo"},
					Offset: 4,
					Length: 3,
				},
			},
		},
		{
			desc: "value with type",
			give: "type Bar int\n" +
				"const Foo Bar = iota\n",
			topLevel: []string{"Foo", "Bar"},
			want:     `const Foo Bar = iota`,
			regions: []Region{
				{
					Label:  &DeclLabel{Name: "Foo"},
					Offset: 6,
					Length: 3,
				},
				{
					Label:  &EntityRefLabel{Name: "Bar"},
					Offset: 10,
					Length: 3,
				},
				{
					Label: &EntityRefLabel{
						ImportPath: Builtin,
						Name:       "iota",
					},
					Offset: 16,
					Length: 4,
				},
			},
		},
		{
			desc: "struct with comment",
			give: "type Foo struct {\n" +
				"	// Bar does things.\n" +
				"	Bar string\n" +
				"}",
			topLevel: []string{"Foo"},
			want: "type Foo struct {\n" +
				"	// Bar does things.\n" +
				"	Bar string\n" +
				"}",
			regions: []Region{
				{
					Label:  &CommentLabel{},
					Offset: 19,
					Length: 19,
				},
				{
					Label: &DeclLabel{
						Parent: "Foo",
						Name:   "Bar",
					},
					Offset: 40,
					Length: 3,
				},
				{
					Label: &EntityRefLabel{
						ImportPath: Builtin,
						Name:       "string",
					},
					Offset: 44,
					Length: 6,
				},
			},
		},
		{
			desc: "imported",
			give: "type Foo bar.Baz",
			imports: map[string]string{
				"example.com/bar-go": "bar",
			},
			topLevel: []string{"Foo"},
			want:     "type Foo bar.Baz",
			regions: []Region{
				{
					Label: &PackageRefLabel{
						ImportPath: "example.com/bar-go",
					},
					Offset: 9,
					Length: 3,
				},
				{
					Label: &EntityRefLabel{
						ImportPath: "example.com/bar-go",
						Name:       "Baz",
					},
					Offset: 13,
					Length: 3,
				},
			},
		},
		{
			desc: "type params",
			give: "type Bar interface{ X() }\n" +
				"type Foo[T Bar] struct{ Field T }",
			topLevel: []string{"Foo", "Bar"},
			want:     "type Foo[T Bar] struct{ Field T }",
			regions: []Region{
				{
					Label:  &EntityRefLabel{Name: "Bar"},
					Offset: 11,
					Length: 3,
				},
				{
					Label: &DeclLabel{
						Parent: "Foo",
						Name:   "Field",
					},
					Offset: 24,
					Length: 5,
				},
			},
		},
		{
			desc: "non-package ref",
			give: `var foo = struct{ Name string }{Name: "foo"}` + "\n" +
				"var Foo = foo.Name",
			topLevel: []string{"Foo"},
			want:     "var Foo = foo.Name",
			regions: []Region{
				{
					Label:  &DeclLabel{Name: "Foo"},
					Offset: 4,
					Length: 3,
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var input strings.Builder
			fmt.Fprintln(&input, "package foo")
			for path := range tt.imports {
				fmt.Fprintf(&input, "import %q\n", path)
			}
			fmt.Fprintln(&input, tt.give)

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "a.go", input.String(), parser.ParseComments)
			require.NoError(t, err)
			require.NotEmpty(t, file.Decls)

			// Connect idents to objects.
			ast.NewPackage(fset, map[string]*ast.File{
				"a.go": file,
			}, func(imports map[string]*ast.Object, path string) (pkg *ast.Object, err error) {
				if o, ok := imports[path]; ok {
					return o, nil
				}

				name, ok := tt.imports[path]
				if !ok {
					return nil, fmt.Errorf("unknown import %q", path)
				}

				o := ast.NewObj(ast.Pkg, name)
				o.Data = ast.NewScope(nil)
				imports[path] = o
				return o, nil
			}, nil)

			df := NewDeclFormatter(fset, tt.topLevel)
			df.Debug(true)
			src, gotRegions, err := df.FormatDecl(file.Decls[len(file.Decls)-1])
			require.NoError(t, err)

			assert.Equal(t, tt.want, string(src))
			assert.Equal(t, tt.regions, gotRegions)
		})
	}
}

// func TestDeclFormatter_packageEntityRef(t *testing.T) {
// 	fset := token.NewFileSet()
// 	fname := filepath.Join("testdata", "package_importer.go")
// 	file, err := parser.ParseFile(fset, fname, nil, parser.ParseComments)
// 	require.NoError(t, err)

// 	ast.NewPackage(fset, map[string]*ast.File{
// 		fname: file,
// 	}, func(imports map[string]*ast.Object, path string) (pkg *ast.Object, err error) {
// 		if o, ok := imports[path]; ok {
// 			return o, nil
// 		}

// 		if path != "example.com/service-go" {
// 			return nil, fmt.Errorf("unknown import %q", path)
// 		}

// 		o := ast.NewObj(ast.Pkg, "service")
// 		o.Data = ast.NewScope(nil)
// 		imports[path] = o
// 		return o, nil
// 	}, nil)

// 	require.NotEmpty(t, file.Decls)

// 	df := NewDeclFormatter(fset, []string{"Handler"})
// 	df.Debug(true)
// 	src, gotRegions, err := df.FormatDecl(file.Decls[len(file.Decls)-1])
// 	require.NoError(t, err)

// 	assert.Empty(t, src)
// 	assert.Empty(t, gotRegions)
// }
