package main

import (
	"go/doc/comment"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocLinker(t *testing.T) {
	t.Parallel()

	var linker docLinker
	linker.LocalPackage("example.com/foo")
	linker.LocalPackage("example.com/bar")
	linker.Template("foo.whatever/baz/qux",
		requireParseTemplate(t, "https://mygodoc.example.com/{{.ImportPath}}"))
	linker.Template("foo.whatever/baz/qux/quux",
		requireParseTemplate(t, "https://godocs.io/{{.ImportPath}}"))

	tests := []struct {
		desc string
		from string
		link comment.DocLink
		want string
	}{
		{
			desc: "default/another package",
			link: comment.DocLink{
				ImportPath: "golang.org/x/net",
			},
			want: "https://pkg.go.dev/golang.org/x/net",
		},
		{
			desc: "entity in another package",
			link: comment.DocLink{
				ImportPath: "golang.org/x/net/context",
				Name:       "Context",
			},
			want: "https://pkg.go.dev/golang.org/x/net/context#Context",
		},
		{
			desc: "method in another package",
			link: comment.DocLink{
				ImportPath: "golang.org/x/tools/go/packages",
				Recv:       "Package",
				Name:       "String",
			},
			want: "https://pkg.go.dev/golang.org/x/tools/go/packages#Package.String",
		},
		{
			desc: "entity in the same package",
			link: comment.DocLink{
				Name: "Foo",
			},
			want: "#Foo",
		},
		{
			desc: "method in the same package",
			link: comment.DocLink{
				Recv: "Foo",
				Name: "Bar",
			},
			want: "#Foo.Bar",
		},
		{
			desc: "local package",
			from: "example.com/foo",
			link: comment.DocLink{
				ImportPath: "example.com/bar",
			},
			want: "../bar",
		},
		{
			desc: "local package/unknown submodule",
			from: "example.com/foo",
			link: comment.DocLink{
				ImportPath: "example.com/bar/baz",
			},
			want: "https://pkg.go.dev/example.com/bar/baz",
		},
		{
			desc: "template",
			link: comment.DocLink{
				ImportPath: "foo.whatever/baz/qux",
			},
			want: "https://mygodoc.example.com/foo.whatever/baz/qux",
		},
		{
			desc: "template/subpackage",
			link: comment.DocLink{
				ImportPath: "foo.whatever/baz/qux/foo",
			},
			want: "https://mygodoc.example.com/foo.whatever/baz/qux/foo",
		},
		{
			desc: "template/override",
			link: comment.DocLink{
				ImportPath: "foo.whatever/baz/qux/quux",
			},
			want: "https://godocs.io/foo.whatever/baz/qux/quux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got := linker.DocLinkURL(tt.from, &tt.link)
			assert.Equal(t, tt.want, got)
		})
	}
}

func requireParseTemplate(t *testing.T, s string) *template.Template {
	tmpl, err := template.New("").Parse(s)
	require.NoError(t, err)
	return tmpl
}
