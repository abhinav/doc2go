package godoc

import (
	"bytes"
	"errors"
	"go/ast"
	"go/doc/comment"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"strings"
	"testing"

	"braces.dev/errtrace"
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
		{
			desc: "example/full-file",
			give: srcPackage{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Lines:      []string{"package foo"},
				TestLines: []string{
					"package foo_test",
					"",
					`import "example.com/foo"`,
					"",
					"func Example() {",
					"	foo.Foo(callback)",
					"	// Output:",
					"	// Hello, world!",
					"}",
					"",
					"func callback() {",
					"	// do something",
					"}",
				},
			},
			want: Package{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Import:     plainCode(`import "example.com/foo"`),
				Examples: []*Example{
					{
						Code: plainCode(
							"package main",
							"",
							`import "example.com/foo"`,
							"",
							"func main() {",
							"	foo.Foo(callback)",
							"}",
							"",
							"func callback() {",
							"	// do something",
							"}",
						),
						Output: "Hello, world!\n",
					},
				},
			},
		},
		{
			desc: "example blocks",
			give: srcPackage{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Lines: []string{
					"package foo",
					"",
					"func Bar() {",
					"	// do something",
					"}",
					"",
					"type Baz struct{}",
					"",
					"func (b *Baz) Quux() {",
					"	// do something",
					"}",
				},
				TestLines: []string{
					"package foo",
					"",
					"// Package-level example demonstrates how to use the package.",
					"func Example() {",
					"	Foo()",
					"",
					"	// Output:",
					"	// Hello, world!",
					"}",
					"",
					"func Example_withSuffix() {", // with suffix
					"	Foo(2)",
					"}",
					"",
					"// This example has unordered output.",
					"func ExampleBar_unorderedOutput() {", // function-level
					"	Bar()",
					"	// Unordered output:",
					"	// Hello",
					"	// World",
					"}",
					"",
					"func ExampleBaz() {", // type-level
					"	fmt.Println(Baz{})",
					"	// Output:",
					"	// {}",
					"}",
					"",
					"func ExampleBaz_Quux_callback() {", // method-level
					"	new(Baz).Quux(func() {",
					"		// do stuff here",
					"	})",
					"}",
				},
			},
			want: Package{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Import:     plainCode(`import "example.com/foo"`),
				Examples: []*Example{
					{
						Code:   plainCode("Foo()"),
						Output: "Hello, world!\n",
						Doc:    commentDoc("Package-level example demonstrates how to use the package."),
					},
					{
						Suffix: "WithSuffix",
						Code:   plainCode("Foo(2)"),
					},
				},
				Functions: []*Function{
					{
						Name:      "Bar",
						ShortDecl: "func Bar()",
						Decl:      plainCode("func Bar()"),
						Examples: []*Example{
							{
								Parent: ExampleParent{Name: "Bar"},
								Code:   plainCode("Bar()"),
								Output: "Hello\nWorld\n",
								Suffix: "UnorderedOutput",
								Doc:    commentDoc("This example has unordered output."),
							},
						},
					},
				},
				Types: []*Type{
					{
						Name: "Baz",
						Decl: plainCode("type Baz struct{}"),
						Examples: []*Example{
							{
								Parent: ExampleParent{Name: "Baz"},
								Code:   plainCode("fmt.Println(Baz{})"),
								Output: "{}\n",
							},
						},
						Methods: []*Function{
							{
								Recv:      "*Baz",
								RecvType:  "Baz",
								Name:      "Quux",
								ShortDecl: "func (b *Baz) Quux()",
								Decl:      plainCode("func (b *Baz) Quux()"),
								Examples: []*Example{
									{
										Parent: ExampleParent{Name: "Quux", Recv: "Baz"},
										Code: plainCode(
											"new(Baz).Quux(func() {",
											"	// do stuff here",
											"})",
										),
										Suffix: "Callback",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc: "deprecated",
			give: srcPackage{
				Name:       "pkg",
				ImportPath: "example.com/pkg",
				Lines: []string{
					"package pkg",
					"",
					"// V is a variable.",
					"//",
					"// Deprecated: use W instead.",
					"var V = 42",
					"",
					"// C is a constant.",
					"//",
					"// Deprecated: use D instead.",
					`const C = "hello"`,
					"",
					"// F is a function.",
					"//",
					"// Deprecated: use G instead.",
					"func F() {}",
					"",
					"// T is a type.",
					"//",
					"// Deprecated: use U instead.",
					"type T struct{}",
					"",
					"// M is a method.",
					"//",
					"// Deprecated: use N instead.",
					"func (t *T) M() {}",
				},
			},
			want: Package{
				Name:       "pkg",
				ImportPath: "example.com/pkg",
				Import:     plainCode(`import "example.com/pkg"`),
				Variables: []*Value{
					{
						Names: []string{"V"},
						Doc: commentDoc(
							"V is a variable.",
							"",
							"Deprecated: use W instead.",
						),
						Decl:       plainCode("var V = 42"),
						Deprecated: true,
					},
				},
				Constants: []*Value{
					{
						Names: []string{"C"},
						Doc: commentDoc(
							"C is a constant.",
							"",
							"Deprecated: use D instead.",
						),
						Decl:       plainCode(`const C = "hello"`),
						Deprecated: true,
					},
				},
				Functions: []*Function{
					{
						Name:      "F",
						ShortDecl: "func F()",
						Decl:      plainCode("func F()"),
						Doc: commentDoc(
							"F is a function.",
							"",
							"Deprecated: use G instead.",
						),
						Deprecated: true,
					},
				},
				Types: []*Type{
					{
						Name: "T",
						Decl: plainCode("type T struct{}"),
						Doc: commentDoc(
							"T is a type.",
							"",
							"Deprecated: use U instead.",
						),
						Deprecated: true,
						Methods: []*Function{
							{
								Recv:      "*T",
								RecvType:  "T",
								Name:      "M",
								ShortDecl: "func (t *T) M()",
								Decl:      plainCode("func (t *T) M()"),
								Doc: commentDoc(
									"M is a method.",
									"",
									"Deprecated: use N instead.",
								),
								Deprecated: true,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got, err := (&Assembler{
				Linker:           &exampleLinker{},
				Lexer:            &nopLexer{},
				newDeclFormatter: newPlainDeclFormatter,
			}).Assemble(tt.give.Build(t))
			require.NoError(t, err)

			got.AllExamples = nil // easier test assertions
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
						{Type: chroma.Whitespace, Value: " "},
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
						{Type: chroma.Whitespace, Value: " "},
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
						{Type: chroma.Whitespace, Value: " "},
						{Type: chroma.NameOther, Value: "foo"},
						{Type: chroma.Whitespace, Value: " "},
						{Type: chroma.LiteralString, Value: `"example.com/foo/v2"`},
					},
				},
			},
		},
	}

	for _, tt := range tests {
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

// Verify that lexing errors are handled gracefully.
func TestAssembler_lexErrors(t *testing.T) {
	pkg := srcPackage{
		Name:       "foo",
		ImportPath: "example.com/foo",
		Lines: []string{
			"package foo",
			"",
			"// Foo does things.",
			"func Foo() {}",
			"",
			"type Bar struct{}",
			"",
			"// Bar does things.",
			"func (b *Bar) Bar() {}",
		},
		TestLines: []string{
			"package foo_test",
			"",
			`import "example.com/foo"`,
			"",
			"func Example() {}",
			"",
			"func ExampleFoo() {}",
			"",
			"func ExampleBar() {}",
			"",
			"func ExampleBar_Bar() {}",
		},
	}

	var buff bytes.Buffer
	_, err := (&Assembler{
		Linker: &exampleLinker{},
		Lexer: &stubLexer{
			Err: errors.New("great sadness"),
		},
		Logger:           log.New(&buff, "", 0),
		newDeclFormatter: newPlainDeclFormatter,
	}).Assemble(pkg.Build(t))
	require.NoError(t, err)

	logs := buff.String()
	assert.Contains(t, logs, "Could not format example")
	assert.Contains(t, logs, "great sadness")
}

// From https://github.com/golang/pkgsite/blob/545ce2ad0d6748cdadb8350c13acc76447df90fd/internal/godoc/dochtml/deprecated_test.go#L9
func TestIsDeprecated(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"A comment", false},
		{"Deprecated: foo", true},
		{" A comment\n   Deprecated: foo", false},
		{" A comment\n\n   Deprecated: foo", true},
		{"This is\n Deprecated.", false},
		{"line 1\nDeprecated:\nline 2\n", false},
		{"line 1\n\nDeprecated:\nline 2\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			assert.Equal(t, tt.want, isDeprecated(tt.text))
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
	return buff.Bytes(), nil, errtrace.Wrap(err)
}

type nopLexer struct{}

var _ highlight.Lexer = (*nopLexer)(nil)

func (l *nopLexer) Lex(bs []byte) ([]chroma.Token, error) {
	return []chroma.Token{
		{
			Type:  chroma.None,
			Value: string(bs),
		},
	}, nil
}

type stubLexer struct {
	Result []chroma.Token
	Err    error
}

var _ highlight.Lexer = (*stubLexer)(nil)

func (l *stubLexer) Lex([]byte) ([]chroma.Token, error) {
	return l.Result, errtrace.Wrap(l.Err)
}

type srcPackage struct {
	Name       string
	ImportPath string
	Lines      []string
	TestLines  []string
}

func (b srcPackage) Build(t *testing.T) *gosrc.Package {
	fset := token.NewFileSet()
	src := strings.Join(b.Lines, "\n") + "\n"
	f, err := parser.ParseFile(fset, "file.go", src, parser.ParseComments)
	require.NoError(t, err)

	var testSyntax []*ast.File
	if len(b.TestLines) > 0 {
		testSrc := strings.Join(b.TestLines, "\n") + "\n"
		f, err := parser.ParseFile(fset, "file_test.go", testSrc, parser.ParseComments)
		require.NoError(t, err)
		testSyntax = []*ast.File{f}
	}

	return &gosrc.Package{
		Name:       b.Name,
		ImportPath: b.ImportPath,
		Syntax:     []*ast.File{f},
		TestSyntax: testSyntax,
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
			&highlight.TokenSpan{
				Tokens: []chroma.Token{
					{
						Type:  chroma.None,
						Value: src,
					},
				},
			},
		},
	}
}
