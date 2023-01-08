package html

import (
	"bytes"
	"go/doc/comment"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	ttemplate "text/template"

	"github.com/andybalholm/cascadia"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/godoc"
	"golang.org/x/net/html"
)

func TestRenderer_WriteStatic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	new(Renderer).WriteStatic(dir)

	var want []string
	err := fs.WalkDir(_staticFS, "static", func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		want = append(want, strings.TrimPrefix(path, "static"))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(want)

	var got []string
	err = fs.WalkDir(os.DirFS(dir), "_", func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		got = append(got, strings.TrimPrefix(path, "_"))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(got)

	assert.Equal(t, want, got)
}

func TestRenderer_WriteStatic_embedded(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	(&Renderer{Embedded: true}).WriteStatic(dir)

	ents, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, ents)
}

func TestRenderer_RenderPackage_title(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc          string
		give          godoc.Package
		wantHeadTitle string // contents of <title>
		wantBodyTitle string // page header
	}{
		{
			desc: "library",
			give: godoc.Package{
				Name:       "foo",
				ImportPath: "example.com/foo",
			},
			wantHeadTitle: "foo",
			wantBodyTitle: "package foo",
		},
		{
			desc: "binary",
			give: godoc.Package{
				Name:       "main",
				ImportPath: "example.com/cmd/foo",
				BinName:    "foo",
			},
			wantHeadTitle: "foo",
			wantBodyTitle: "foo",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			pinfo := PackageInfo{
				Package:    &tt.give,
				DocPrinter: new(CommentDocPrinter),
			}

			var buff bytes.Buffer
			require.NoError(t,
				new(Renderer).RenderPackage(&buff, &pinfo))

			doc, err := html.Parse(bytes.NewReader(buff.Bytes()))
			require.NoError(t, err, "invalid HTML:\n%v", buff.String())

			headTitle := querySelector(doc, "title")
			require.NotNil(t, headTitle)
			assert.Equal(t, tt.wantHeadTitle, allText(headTitle))

			bodyTitle := querySelector(doc, "#pkg-overview")
			require.NotNil(t, bodyTitle)
			assert.Equal(t, tt.wantBodyTitle, allText(bodyTitle))
		})
	}
}

func TestRenderPackage_index(t *testing.T) {
	t.Parallel()

	type testCase struct {
		desc string
		give godoc.Package
		want []string
	}

	tests := []testCase{
		{desc: "empty"},
		{
			desc: "constants",
			give: godoc.Package{
				Constants: []*godoc.Value{
					{
						Names: []string{"Foo"},
						Decl:  textSpan("var Foo = 42"),
					},
				},
			},
			want: []string{"Constants"},
		},
		{
			desc: "variables",
			give: godoc.Package{
				Variables: []*godoc.Value{
					{
						Names: []string{"Foo"},
						Decl:  textSpan("var Foo = 42"),
					},
				},
			},
			want: []string{"Variables"},
		},
		{
			desc: "functions",
			give: godoc.Package{
				Functions: []*godoc.Function{
					{
						Name:      "Foo",
						Decl:      textSpan("func Foo()"),
						ShortDecl: "func Foo()",
					},
					{
						Name:      "Bar",
						Decl:      textSpan("func Bar(int) string"),
						ShortDecl: "func Bar(int) string",
					},
				},
			},
			want: []string{"func Foo()", "func Bar(int) string"},
		},
		{
			desc: "types",
			give: godoc.Package{
				Types: []*godoc.Type{
					{
						Name: "Foo",
						Decl: textSpan("type Foo struct{}"),
					},
					{
						Name: "Bar",
						Decl: textSpan("type Bar interface{ Do(Foo) }"),
					},
				},
			},
			want: []string{"type Foo", "type Bar"},
		},
		{
			desc: "type with associated functions",
			give: godoc.Package{
				Types: []*godoc.Type{
					{
						Name: "Foo",
						Decl: textSpan("type Foo struct{}"),
						Functions: []*godoc.Function{
							{
								Name:      "NewFoo",
								Decl:      textSpan("func NewFoo() *Foo"),
								ShortDecl: "func NewFoo() *Foo",
							},
						},
						Methods: []*godoc.Function{
							{
								Name:      "Get",
								Decl:      textSpan("func (f *Foo) Get() string"),
								ShortDecl: "func (f *Foo) Get() string",
								Recv:      "*Foo",
								RecvType:  "Foo",
							},
						},
					},
				},
			},
			want: []string{
				"type Foo",
				"func NewFoo() *Foo",
				"func (f *Foo) Get() string",
			},
		},
	}

	runTest := func(t *testing.T, renderer *Renderer, tt testCase) {
		pinfo := PackageInfo{
			Package:    &tt.give,
			DocPrinter: new(CommentDocPrinter),
		}

		var buff bytes.Buffer
		require.NoError(t,
			renderer.RenderPackage(&buff, &pinfo))

		doc, err := html.Parse(bytes.NewReader(buff.Bytes()))
		require.NoError(t, err, "invalid HTML:\n%v", buff.String())

		index := querySelector(doc, "#pkg-index + ul")
		var items []string
		if index != nil {
			for _, li := range querySelectorAll(index, "li > a") {
				items = append(items, text(li))
			}
		}
		assert.Equal(t, tt.want, items)
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			t.Run("Embedded", func(t *testing.T) {
				t.Parallel()

				runTest(t, &Renderer{Embedded: true}, tt)
			})

			t.Run("Standalone", func(t *testing.T) {
				t.Parallel()

				runTest(t, new(Renderer), tt)
			})
		})
	}
}

func TestRenderPackage_headers(t *testing.T) {
	t.Parallel()

	pkg := godoc.Package{
		Name:       "foo",
		ImportPath: "example.com/foo",
		Doc:        parseDoc("# Some package"),
		Constants: []*godoc.Value{
			{
				Names: []string{"ConstantValue"},
				Decl:  textSpan("const ConstantValue = 42"),
				Doc:   parseDoc("# Some constant"),
			},
		},
		Variables: []*godoc.Value{
			{
				Names: []string{"VariableValue"},
				Decl:  textSpan("var VariableValue = 42"),
				Doc:   parseDoc("# Some variable"),
			},
		},
		Functions: []*godoc.Function{
			{
				Name:      "DoStuff",
				Doc:       parseDoc("# Doer of stuff"),
				Decl:      textSpan("func DoStuff()"),
				ShortDecl: "func DoStuff()",
			},
		},
		Types: []*godoc.Type{
			{
				Name: "SomeType",
				Doc:  parseDoc("# My type"),
				Decl: textSpan("type SomeType string"),
				Constants: []*godoc.Value{
					{
						Names: []string{"DefaultSomeType"},
						Decl:  textSpan(`const DefaultSomeType SomeType = "42"`),
						Doc:   parseDoc("# Default Some Type"),
					},
				},
				Variables: []*godoc.Value{
					{
						Names: []string{"SharedSomeType"},
						Decl:  textSpan(`var SharedSomeType = SomeType("foo")`),
						Doc:   parseDoc("# Shared Some Type"),
					},
				},
				Functions: []*godoc.Function{
					{
						Name:      "NewSomeType",
						Doc:       parseDoc("# Constructor"),
						Decl:      textSpan("func NewSomeType() SomeType"),
						ShortDecl: "func NewSomeType() SomeType",
					},
				},
				Methods: []*godoc.Function{
					{
						Name:      "Print",
						Doc:       parseDoc("# Method"),
						Decl:      textSpan("func (SomeType) Print()"),
						Recv:      "SomeType",
						RecvType:  "SomeType",
						ShortDecl: "func (SomeType) Print()",
					},
				},
			},
		},
	}
	pinfo := PackageInfo{
		Package:    &pkg,
		DocPrinter: new(CommentDocPrinter),
	}

	var buff bytes.Buffer
	require.NoError(t, new(Renderer).RenderPackage(&buff, &pinfo))

	doc, err := html.Parse(bytes.NewReader(buff.Bytes()))
	require.NoError(t, err, "invalid HTML:\n%v", buff.String())

	type header struct {
		level int
		id    string
		body  string
	}

	var headers []header
	for _, h := range querySelectorAll(doc, "h1, h2, h3, h4, h5, h6") {
		lvl, err := strconv.Atoi(strings.TrimPrefix(h.Data, "h"))
		require.NoError(t, err, "Could not determine level of <%v>", h.Data)

		headers = append(headers, header{
			level: lvl,
			id:    attr(h, "id"),
			body:  allText(h),
		})
	}

	assert.Equal(t, []header{
		{2, "pkg-overview", "package foo"},
		{3, "hdr-Some_package", "Some package"},
		{3, "pkg-index", "Index"},
		{3, "pkg-constants", "Constants"},
		{4, "hdr-Some_constant", "Some constant"},
		{3, "pkg-variables", "Variables"},
		{4, "hdr-Some_variable", "Some variable"},
		{3, "pkg-functions", "Functions"},
		{3, "DoStuff", "func DoStuff"},
		{4, "hdr-Doer_of_stuff", "Doer of stuff"},
		{3, "pkg-types", "Types"},
		{3, "SomeType", "type SomeType"},
		{4, "hdr-My_type", "My type"},
		{4, "hdr-Default_Some_Type", "Default Some Type"},
		{4, "hdr-Shared_Some_Type", "Shared Some Type"},
		{4, "NewSomeType", "func NewSomeType"},
		{5, "hdr-Constructor", "Constructor"},
		{4, "SomeType.Print", "func (SomeType) Print"},
		{5, "hdr-Method", "Method"},
	}, headers)
}

func TestRenderSubpackages(t *testing.T) {
	t.Parallel()

	type link struct {
		href     string
		synopsis string
	}

	tests := []struct {
		desc     string
		internal bool
		subpkgs  []Subpackage
		want     []link
	}{
		{
			desc:     "internal",
			internal: true,
			subpkgs: []Subpackage{
				{
					RelativePath: "internal/foo",
					Synopsis:     "Does things with foo",
				},
				{
					RelativePath: "bar",
					Synopsis:     "Public package bar",
				},
			},
			want: []link{
				{"internal/foo", "Does things with foo"},
				{"bar", "Public package bar"},
			},
		},
		{
			desc:     "no internal",
			internal: false,
			subpkgs: []Subpackage{
				{
					RelativePath: "internal/foo",
					Synopsis:     "Does things with foo",
				},
				{
					RelativePath: "bar",
					Synopsis:     "Public package bar",
				},
			},
			want: []link{
				{"bar", "Public package bar"},
			},
		},
	}

	assertLinks := func(t *testing.T, want []link, output []byte) {
		doc, err := html.Parse(bytes.NewReader(output))
		require.NoError(t, err, "invalid HTML:\n%s", output)
		assert.Contains(t, string(output), "Directories")

		table := querySelector(doc, "#pkg-directories + table")
		require.NotNil(t, table, "pkg-directories not found:\n%s", output)

		var got []link
		for _, tr := range querySelectorAll(table, "tbody > tr") {
			got = append(got, link{
				href:     attr(querySelector(tr, "td > a"), "href"),
				synopsis: text(querySelector(tr, "td + td")),
			})
		}

		assert.Equal(t, want, got)
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Run("package", func(t *testing.T) {
				pinfo := PackageInfo{
					Package: &godoc.Package{
						Name:       "foo",
						ImportPath: "example.com/foo/bar/baz",
					},
					DocPrinter:  new(CommentDocPrinter),
					Subpackages: tt.subpkgs,
				}

				var buff bytes.Buffer
				require.NoError(t, (&Renderer{
					Internal: tt.internal,
				}).RenderPackage(&buff, &pinfo))

				assertLinks(t, tt.want, buff.Bytes())
			})

			t.Run("directory", func(t *testing.T) {
				pidx := PackageIndex{
					Path:        "example.com/foo/bar/baz",
					Subpackages: tt.subpkgs,
				}

				var buff bytes.Buffer
				require.NoError(t, (&Renderer{
					Internal: tt.internal,
				}).RenderPackageIndex(&buff, &pidx))

				assertLinks(t, tt.want, buff.Bytes())
			})
		})
	}
}

// If all we have is internal subpackages,
// and we're not rendering internal packages,
// don't generate a subpackages section.
func TestRenderSubpackages_skipEmptyInternal(t *testing.T) {
	subpackages := []Subpackage{
		{RelativePath: "internal/foo"},
		{RelativePath: "internal/bar"},
		{RelativePath: "internal/baz"},
	}

	assertNoSubpackages := func(t *testing.T, output []byte) {
		doc, err := html.Parse(bytes.NewReader(output))
		require.NoError(t, err, "invalid HTML:\n%s", output)

		h := querySelector(doc, "#pkg-directories")
		assert.Nil(t, h, "There should be no pkg-directories:\n%s", output)
		assert.NotContains(t, string(output), "Directories")
	}

	t.Run("package", func(t *testing.T) {
		pinfo := PackageInfo{
			Package: &godoc.Package{
				Name:       "foo",
				ImportPath: "example.com/fo",
			},
			DocPrinter:  new(CommentDocPrinter),
			Subpackages: subpackages,
		}

		var buff bytes.Buffer
		require.NoError(t, new(Renderer).RenderPackage(&buff, &pinfo))
		assertNoSubpackages(t, buff.Bytes())
	})

	t.Run("directory", func(t *testing.T) {
		pidx := PackageIndex{
			Path:        "example.com/foo",
			Subpackages: subpackages,
		}

		var buff bytes.Buffer
		require.NoError(t, new(Renderer).RenderPackageIndex(&buff, &pidx))
		assertNoSubpackages(t, buff.Bytes())
	})
}

func TestRenderBreadcrumbs(t *testing.T) {
	t.Parallel()

	crumbs := []Breadcrumb{
		{Text: "example.com", Path: "example.com"},
		{Text: "foo", Path: "example.com/foo"},
		{Text: "bar", Path: "example.com/foo/bar"},
	}

	type link struct {
		href string
		body string
	}

	wantLinks := []link{
		{"../../..", "example.com"},
		{"../..", "foo"},
		{"..", "bar"},
	}

	assertCrumbs := func(t *testing.T, output []byte) {
		doc, err := html.Parse(bytes.NewReader(output))
		require.NoError(t, err, "invalid HTML:\n%s", output)

		var got []link
		for _, a := range querySelectorAll(doc, "nav > a") {
			got = append(got, link{
				href: attr(a, "href"),
				body: text(a),
			})
		}

		assert.Equal(t, wantLinks, got)
	}

	t.Run("package", func(t *testing.T) {
		pinfo := PackageInfo{
			Package: &godoc.Package{
				Name:       "foo",
				ImportPath: "example.com/foo/bar/baz",
			},
			DocPrinter:  new(CommentDocPrinter),
			Breadcrumbs: crumbs,
		}

		var buff bytes.Buffer
		require.NoError(t, new(Renderer).RenderPackage(&buff, &pinfo))
		assertCrumbs(t, buff.Bytes())
	})

	t.Run("directory", func(t *testing.T) {
		pidx := PackageIndex{
			Path: "example.com/foo/bar/baz",
			Breadcrumbs: []Breadcrumb{
				{Text: "example.com", Path: "example.com"},
				{Text: "foo", Path: "example.com/foo"},
				{Text: "bar", Path: "example.com/foo/bar"},
			},
		}
		var buff bytes.Buffer
		require.NoError(t, new(Renderer).RenderPackageIndex(&buff, &pidx))
		assertCrumbs(t, buff.Bytes())
	})
}

func TestFrontmatter(t *testing.T) {
	t.Parallel()

	smallPkg := PackageInfo{
		NumChildren: 5,
		Package: &godoc.Package{
			Name:       "foo",
			ImportPath: "example.com/foo",
		},
	}
	smallDir := PackageIndex{
		Path:        "example.com/foo/bar",
		NumChildren: 6,
	}

	tests := []struct {
		desc string
		tmpl string

		// One of the following two must be set.
		pkg *PackageInfo
		dir *PackageIndex

		want string
	}{
		{
			desc: "pkg",
			tmpl: "{{.Path}}\n{{.Basename}}\n{{.NumChildren}}",
			pkg:  &smallPkg,
			want: "example.com/foo\nfoo\n5\n\n",
		},
		{
			desc: "dir",
			tmpl: "{{.Path}}\n{{.Basename}}\n{{.NumChildren}}",
			dir:  &smallDir,
			want: "example.com/foo/bar\nbar\n6\n\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			tmpl, err := ttemplate.New(t.Name()).Parse(tt.tmpl)
			require.NoError(t, err)

			rnd := Renderer{
				FrontMatter: tmpl,
			}

			var buff bytes.Buffer
			if tt.pkg != nil {
				require.NoError(t, rnd.RenderPackage(&buff, tt.pkg))
			} else if tt.dir != nil {
				require.NoError(t, rnd.RenderPackageIndex(&buff, tt.dir))
			} else {
				t.Fatal("Bad test case: one of pkg or dir must be set")
			}

			require.True(t, strings.HasPrefix(buff.String(), tt.want),
				"file must start with %q, got:\n%s", tt.want, buff.String())
		})
	}
}

func TestBasename(t *testing.T) {
	t.Parallel()

	type hasBasename interface{ Basename() string }

	tests := []struct {
		desc string
		give hasBasename
		want string
	}{
		{
			desc: "package",
			give: &PackageInfo{
				Package: &godoc.Package{
					ImportPath: "example.com/foo/bar/baz",
				},
			},
			want: "baz",
		},
		{
			desc: "directory",
			give: &PackageIndex{
				Path: "example.com/foo/bar",
			},
			want: "bar",
		},
		{
			desc: "root directory",
			give: &PackageIndex{},
			want: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.give.Basename())
		})
	}
}

func querySelector(n *html.Node, query string) *html.Node {
	return cascadia.Query(n, cascadia.MustCompile(query))
}

func querySelectorAll(n *html.Node, query string) []*html.Node {
	return cascadia.QueryAll(n, cascadia.MustCompile(query))
}

func allText(n *html.Node) string {
	var (
		sb    strings.Builder
		visit func(*html.Node)
	)
	visit = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for n := n.FirstChild; n != nil; n = n.NextSibling {
			visit(n)
		}
	}
	visit(n)
	return sb.String()
}

func text(n *html.Node) string {
	var sb strings.Builder
	for n := n.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
	}
	return sb.String()
}

func textSpan(str string) *godoc.Code {
	return &godoc.Code{
		Spans: []godoc.Span{
			&godoc.TextSpan{
				Text: []byte(str),
			},
		},
	}
}

func parseDoc(s string) *comment.Doc {
	return new(comment.Parser).Parse(s)
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
