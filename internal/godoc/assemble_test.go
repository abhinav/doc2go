package godoc

import (
	"bytes"
	"errors"
	"go/ast"
	"go/doc/comment"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/highlight"
)

func TestAssembler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give srcPackage
		want Package
	}{
		{
			desc: "minimal",
			give: srcPackage{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Lines: []string{
					"// Package foo does stuff.",
					"// This is another line.",
					"package foo",
				},
			},
			want: Package{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Import:     plainCode(`import "example.com/foo"`),
				Doc: commentDoc(
					"Package foo does stuff.",
					"This is another line.",
				),
				Synopsis: "Package foo does stuff.",
			},
		},
		{
			desc: "binary",
			give: srcPackage{
				Name:       "main",
				ImportPath: "example.com/cmd/foo",
				Lines: []string{
					"// foo is a CLI.",
					"package main",
				},
			},
			want: Package{
				Name:       "main",
				BinName:    "foo",
				ImportPath: "example.com/cmd/foo",
				Import:     plainCode(`import "example.com/cmd/foo"`),
				Doc:        commentDoc("foo is a CLI."),
				Synopsis:   "foo is a CLI.",
			},
		},
		{
			desc: "constants",
			give: srcPackage{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Lines: []string{
					"package foo",
					"",
					"// Foo is a constant",
					"const Foo = 42",
				},
			},
			want: Package{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Import:     plainCode(`import "example.com/foo"`),
				Constants: []*Value{
					{
						Names: []string{"Foo"},
						Doc:   commentDoc("Foo is a constant"),
						Decl:  plainCode("const Foo = 42"),
					},
				},
			},
		},
		{
			desc: "variables",
			give: srcPackage{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Lines: []string{
					"package foo",
					"",
					"// Err is an error.",
					`var Err = errors.New("great sadness")`,
				},
			},
			want: Package{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Import:     plainCode(`import "example.com/foo"`),
				Variables: []*Value{
					{
						Names: []string{"Err"},
						Doc:   commentDoc("Err is an error."),
						Decl:  plainCode(`var Err = errors.New("great sadness")`),
					},
				},
			},
		},
		{
			desc: "type",
			give: srcPackage{
				Name:       "bar",
				ImportPath: "example.com/bar",
				Lines: []string{
					"package bar",
					"// Foo is an empty struct.",
					"type Foo struct{}",
				},
			},
			want: Package{
				Name:       "bar",
				ImportPath: "example.com/bar",
				Import:     plainCode(`import "example.com/bar"`),
				Types: []*Type{
					{
						Name: "Foo",
						Doc:  commentDoc("Foo is an empty struct."),
						Decl: plainCode("type Foo struct{}"),
					},
				},
			},
		},
		{
			desc: "function",
			give: srcPackage{
				Name:       "bar",
				ImportPath: "example.com/bar",
				Lines: []string{
					"package bar",
					"// Foo does things.",
					"func Foo() {}",
				},
			},
			want: Package{
				Name:       "bar",
				ImportPath: "example.com/bar",
				Import:     plainCode(`import "example.com/bar"`),
				Functions: []*Function{
					{
						Name:      "Foo",
						Doc:       commentDoc("Foo does things."),
						Decl:      plainCode("func Foo()"),
						ShortDecl: "func Foo()",
					},
				},
			},
		},
		{
			desc: "type/constant",
			give: srcPackage{
				Name:       "bar",
				ImportPath: "example.com/bar",
				Lines: []string{
					"package bar",
					"",
					"// Role specifies a user's abilities.",
					"type Role int",
					"",
					"// Various supported roles.",
					"const (",
					"	User Role = iota",
					"	Mod",
					"	Admin",
					")",
				},
			},
			want: Package{
				Name:       "bar",
				ImportPath: "example.com/bar",
				Import:     plainCode(`import "example.com/bar"`),
				Types: []*Type{
					{
						Name: "Role",
						Doc:  commentDoc("Role specifies a user's abilities."),
						Decl: plainCode("type Role int"),
						Constants: []*Value{
							{
								Names: []string{"User", "Mod", "Admin"},
								Doc:   commentDoc("Various supported roles."),
								Decl: plainCode(
									"const (",
									"	User Role = iota",
									"	Mod",
									"	Admin",
									")",
								),
							},
						},
					},
				},
			},
		},
		{
			desc: "type/variable",
			give: srcPackage{
				Name:       "flag",
				ImportPath: "example.com/flag",
				Lines: []string{
					"package flag",
					"",
					"// FlagSet defines a flag grouping.",
					"type FlagSet struct{ impl *stuff }",
					"",
					"// DefaultFlagSet is the default group of flags.",
					"var DefaultFlagSet FlagSet = newFlagSet()",
				},
			},
			want: Package{
				Name:       "flag",
				ImportPath: "example.com/flag",
				Import:     plainCode(`import "example.com/flag"`),
				Types: []*Type{
					{
						Name: "FlagSet",
						Doc:  commentDoc("FlagSet defines a flag grouping."),
						Decl: plainCode(
							"type FlagSet struct {",
							"	// contains filtered or unexported fields",
							"}",
						),
						Variables: []*Value{
							{
								Names: []string{"DefaultFlagSet"},
								Doc:   commentDoc("DefaultFlagSet is the default group of flags."),
								Decl:  plainCode("var DefaultFlagSet FlagSet = newFlagSet()"),
							},
						},
					},
				},
			},
		},
		{
			desc: "type/function",
			give: srcPackage{
				Name:       "flag",
				ImportPath: "example.com/flag",
				Lines: []string{
					"package flag",
					"",
					"type FlagSet struct{}",
					"",
					"// NewFlagSet builds a new FlagSet.",
					"func NewFlagSet() *FlagSet {",
					"	return &FlagSet{}",
					"}",
				},
			},
			want: Package{
				Name:       "flag",
				ImportPath: "example.com/flag",
				Import:     plainCode(`import "example.com/flag"`),
				Types: []*Type{
					{
						Name: "FlagSet",
						Decl: plainCode("type FlagSet struct{}"),
						Functions: []*Function{
							{
								Name:      "NewFlagSet",
								Doc:       commentDoc("NewFlagSet builds a new FlagSet."),
								Decl:      plainCode("func NewFlagSet() *FlagSet"),
								ShortDecl: "func NewFlagSet() *FlagSet",
							},
						},
					},
				},
			},
		},
		{
			desc: "type/method",
			give: srcPackage{
				Name:       "flag",
				ImportPath: "example.com/flag",
				Lines: []string{
					"package flag",
					"",
					"type FlagSet struct{}",
					"",
					"// Bool registers a new boolean flag.",
					"func (f *FlagSet) Bool(name string, value bool, usage string) *bool {",
					`	panic("not yet implemented")`,
					"}",
				},
			},
			want: Package{
				Name:       "flag",
				ImportPath: "example.com/flag",
				Import:     plainCode(`import "example.com/flag"`),
				Types: []*Type{
					{
						Name: "FlagSet",
						Decl: plainCode("type FlagSet struct{}"),
						Methods: []*Function{
							{
								Recv:      "*FlagSet",
								RecvType:  "FlagSet",
								Name:      "Bool",
								Doc:       commentDoc("Bool registers a new boolean flag."),
								Decl:      plainCode("func (f *FlagSet) Bool(name string, value bool, usage string) *bool"),
								ShortDecl: "func (f *FlagSet) Bool(name string, value bool, usage string) *bool",
							},
						},
					},
				},
			},
		},
		{
			desc: "basename mismatch",
			give: srcPackage{
				Name:       "foo",
				ImportPath: "example.com/foo/v2",
				Lines: []string{
					"package foo",
				},
			},
			want: Package{
				Name:       "foo",
				ImportPath: "example.com/foo/v2",
				Import:     plainCode(`import foo "example.com/foo/v2"`),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got, err := (&Assembler{
				Linker: &exampleLinker{},
				Lexer: &stubLexer{
					Err: errors.New("great sadness"),
				},
				newDeclFormatter: newPlainDeclFormatter,
			}).Assemble(tt.give.Build(t))
			require.NoError(t, err)
			assert.Equal(t, &tt.want, got)
		})
	}
}

func TestAssembler_ImportFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		name       string
		importPath string
		want       []highlight.Span
	}{
		{
			desc:       "default",
			name:       "foo",
			importPath: "example.com/foo",
			want: []highlight.Span{
				&highlight.TokenSpan{
					Tokens: []chroma.Token{
						{Type: chroma.KeywordNamespace, Value: "import"},
						{Type: chroma.Text, Value: " "},
						{Type: chroma.LiteralString, Value: `"example.com/foo"`},
					},
				},
			},
		},
		{
			desc:       "binary",
			name:       "main",
			importPath: "example.com/foo",
			want: []highlight.Span{
				&highlight.TokenSpan{
					Tokens: []chroma.Token{
						{Type: chroma.KeywordNamespace, Value: "import"},
						{Type: chroma.Text, Value: " "},
						{Type: chroma.LiteralString, Value: `"example.com/foo"`},
					},
				},
			},
		},
		{
			desc:       "v2",
			name:       "foo",
			importPath: "example.com/foo/v2",
			want: []highlight.Span{
				&highlight.TokenSpan{
					Tokens: []chroma.Token{
						{Type: chroma.KeywordNamespace, Value: "import"},
						{Type: chroma.Text, Value: " "},
						{Type: chroma.NameOther, Value: "foo"},
						{Type: chroma.Text, Value: " "},
						{Type: chroma.LiteralString, Value: `"example.com/foo/v2"`},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			pkg := srcPackage{
				Name:       tt.name,
				ImportPath: tt.importPath,
				Lines: []string{
					"package " + tt.name,
				},
			}.Build(t)

			got, err := (&Assembler{
				Linker:           &exampleLinker{},
				Lexer:            highlight.GoLexer,
				newDeclFormatter: newPlainDeclFormatter,
			}).Assemble(pkg)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Import.Spans)
		})
	}
}

type exampleLinker struct{}

var _ Linker = (*exampleLinker)(nil)

func (*exampleLinker) DocLinkURL(_ string, link *comment.DocLink) string {
	return link.DefaultURL("https://example.com")
}

// plainDeclFormatter formats source code into plain text exclusively.
type plainDeclFormatter struct{ fset *token.FileSet }

var _ DeclFormatter = (*plainDeclFormatter)(nil)

func newPlainDeclFormatter(pkg *gosrc.Package) DeclFormatter {
	return &plainDeclFormatter{fset: pkg.Fset}
}

func (f *plainDeclFormatter) FormatDecl(decl ast.Decl) ([]byte, []gosrc.Region, error) {
	var buff bytes.Buffer
	err := format.Node(&buff, f.fset, decl)
	return buff.Bytes(), nil, err
}

type stubLexer struct {
	Result []chroma.Token
	Err    error
}

var _ highlight.Lexer = (*stubLexer)(nil)

func (l *stubLexer) Lex([]byte) ([]chroma.Token, error) {
	return l.Result, l.Err
}

type srcPackage struct {
	Name       string
	ImportPath string
	Lines      []string
}

func (b srcPackage) Build(t *testing.T) *gosrc.Package {
	fset := token.NewFileSet()
	src := strings.Join(b.Lines, "\n") + "\n"
	f, err := parser.ParseFile(fset, "file.go", src, parser.ParseComments)
	require.NoError(t, err)

	return &gosrc.Package{
		Name:       b.Name,
		ImportPath: b.ImportPath,
		Syntax:     []*ast.File{f},
		Fset:       fset,
	}
}

func commentDoc(lines ...string) *comment.Doc {
	txt := strings.Join(lines, "\n") + "\n"
	return new(comment.Parser).Parse(txt)
}

func plainCode(lines ...string) *highlight.Code {
	src := strings.Join(lines, "\n")
	return &highlight.Code{
		Spans: []highlight.Span{
			&highlight.TextSpan{
				Text: []byte(src),
			},
		},
	}
}
