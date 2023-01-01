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

func TestRenderPackage_breadcrumbs(t *testing.T) {
	t.Parallel()

	crumbs := []Breadcrumb{
		{Text: "example.com", Path: "example.com"},
		{Text: "foo", Path: "example.com/foo"},
		{Text: "bar", Path: "example.com/foo/bar"},
	}
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

	doc, err := html.Parse(bytes.NewReader(buff.Bytes()))
	require.NoError(t, err, "invalid HTML:\n%v", buff.String())

	type link struct {
		href string
		body string
	}

	var got []link
	for _, a := range querySelectorAll(doc, "nav > a") {
		got = append(got, link{
			href: attr(a, "href"),
			body: text(a),
		})
	}

	assert.Equal(t, []link{
		{"../../..", "example.com"},
		{"../..", "foo"},
		{"..", "bar"},
	}, got)
}

func TestRenderPackage_subpackages(t *testing.T) {
	t.Parallel()

	pinfo := PackageInfo{
		Package: &godoc.Package{
			Name:       "foo",
			ImportPath: "example.com/foo/bar/baz",
		},
		DocPrinter: new(CommentDocPrinter),
		Subpackages: []Subpackage{
			{
				RelativePath: "internal/foo",
				Synopsis:     "Does things with foo",
			},
			{
				RelativePath: "bar",
				Synopsis:     "Public package bar",
			},
		},
	}

	var buff bytes.Buffer
	require.NoError(t, new(Renderer).RenderPackage(&buff, &pinfo))

	doc, err := html.Parse(bytes.NewReader(buff.Bytes()))
	require.NoError(t, err, "invalid HTML:\n%v", buff.String())

	table := querySelector(doc, "#pkg-directories + table")
	require.NotNil(t, table, "pkg-directories not found:\n%v", buff.String())

	type link struct {
		href     string
		synopsis string
	}

	var got []link
	for _, tr := range querySelectorAll(table, "tbody > tr") {
		got = append(got, link{
			href:     attr(querySelector(tr, "td > a"), "href"),
			synopsis: text(querySelector(tr, "td + td")),
		})
	}

	assert.Equal(t, []link{
		{"internal/foo", "Does things with foo"},
		{"bar", "Public package bar"},
	}, got)
}

func TestRenderPackageIndex(t *testing.T) {
	t.Parallel()

	pidx := PackageIndex{
		Path: "example.com/foo/bar/baz",
		Breadcrumbs: []Breadcrumb{
			{Text: "example.com", Path: "example.com"},
			{Text: "foo", Path: "example.com/foo"},
			{Text: "bar", Path: "example.com/foo/bar"},
		},
		Subpackages: []Subpackage{
			{
				RelativePath: "internal/foo",
				Synopsis:     "Does things with foo",
			},
			{
				RelativePath: "bar",
				Synopsis:     "Public package bar",
			},
		},
	}

	var buff bytes.Buffer
	require.NoError(t, new(Renderer).RenderPackageIndex(&buff, &pidx))

	doc, err := html.Parse(bytes.NewReader(buff.Bytes()))
	require.NoError(t, err, "invalid HTML:\n%v", buff.String())

	type crumb struct {
		href string
		body string
	}

	type subdir struct {
		href     string
		synopsis string
	}

	var crumbs []crumb
	for _, a := range querySelectorAll(doc, "nav > a") {
		crumbs = append(crumbs, crumb{
			href: attr(a, "href"),
			body: text(a),
		})
	}

	table := querySelector(doc, "#pkg-directories + table")
	require.NotNil(t, table, "pkg-directories not found:\n%v", buff.String())

	var subdirs []subdir
	for _, tr := range querySelectorAll(table, "tbody > tr") {
		subdirs = append(subdirs, subdir{
			href:     attr(querySelector(tr, "td > a"), "href"),
			synopsis: text(querySelector(tr, "td + td")),
		})
	}

	assert.Equal(t, []crumb{
		{"../../..", "example.com"},
		{"../..", "foo"},
		{"..", "bar"},
	}, crumbs)

	assert.Equal(t, []subdir{
		{"internal/foo", "Does things with foo"},
		{"bar", "Public package bar"},
	}, subdirs)
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
