package html

import (
	"bytes"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/andybalholm/cascadia"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/godoc"
	"golang.org/x/net/html"
)

func TestRenderer_WriteStatic(t *testing.T) {
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
	dir := t.TempDir()
	(&Renderer{Embedded: true}).WriteStatic(dir)

	ents, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, ents)
}

func TestRenderer_RenderPackage_title(t *testing.T) {
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
		t.Run(tt.desc, func(t *testing.T) {
			pinfo := PackageInfo{
				Package:    &tt.give,
				DocPrinter: new(CommentDocPrinter),
			}

			var buff bytes.Buffer
			require.NoError(t,
				new(Renderer).RenderPackage(&buff, &pinfo))

			doc, err := html.Parse(bytes.NewReader(buff.Bytes()))
			require.NoError(t, err, "invalid HTML:\n%v", buff.String())

			headTitle := cascadia.MustCompile("title").MatchFirst(doc)
			require.NotNil(t, headTitle)
			assert.Equal(t, tt.wantHeadTitle, allText(headTitle))

			bodyTitle := cascadia.MustCompile("#pkg-overview").MatchFirst(doc)
			require.NotNil(t, bodyTitle)
			assert.Equal(t, tt.wantBodyTitle, allText(bodyTitle))
		})
	}
}

func TestRenderPackage_index(t *testing.T) {
	tests := []struct {
		desc string
		give godoc.Package
		want []string
	}{
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

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			pinfo := PackageInfo{
				Package:    &tt.give,
				DocPrinter: new(CommentDocPrinter),
			}

			var buff bytes.Buffer
			require.NoError(t,
				new(Renderer).RenderPackage(&buff, &pinfo))

			doc, err := html.Parse(bytes.NewReader(buff.Bytes()))
			require.NoError(t, err, "invalid HTML:\n%v", buff.String())

			index := cascadia.MustCompile("#pkg-index + ul").MatchFirst(doc)
			var items []string
			if index != nil {
				for _, li := range cascadia.QueryAll(index, cascadia.MustCompile("li > a")) {
					items = append(items, text(li))
				}
			}
			assert.Equal(t, tt.want, items)
		})
	}
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
