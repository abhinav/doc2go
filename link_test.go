package main

import (
	"go/doc/comment"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/gomod"
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

func TestDocLinker_templateModuleData(t *testing.T) {
	t.Parallel()

	// Set up a module tree with test dependencies.
	var tree gomod.Tree
	tree.PutModuleDeps("example.com/myproject", []*gomod.Module{
		{Path: "go.uber.org/zap", Version: "v1.27.1"},
		{Path: "github.com/stretchr/testify", Version: "v1.8.4"},
	})

	var linker docLinker
	linker.ModuleTree = &tree
	linker.LocalPackage("example.com/myproject")

	tests := []struct {
		desc     string
		from     string
		pkg      string
		template string
		want     string
	}{
		{
			desc:     "module info available/module root",
			from:     "example.com/myproject",
			pkg:      "go.uber.org/zap",
			template: `{{.Module.Path}}@{{.Module.Version}}`,
			want:     "go.uber.org/zap@v1.27.1",
		},
		{
			desc:     "module info available/subpackage",
			from:     "example.com/myproject",
			pkg:      "go.uber.org/zap/zaptest/observer",
			template: `{{.Module.Path}}@{{.Module.Version}}/{{.Module.Subpath}}`,
			want:     "go.uber.org/zap@v1.27.1/zaptest/observer",
		},
		{
			desc:     "module info available/conditional subpath",
			from:     "example.com/myproject",
			pkg:      "github.com/stretchr/testify/assert",
			template: `{{.Module.Path}}@{{.Module.Version}}{{with .Module.Subpath}}/{{.}}{{end}}`,
			want:     "github.com/stretchr/testify@v1.8.4/assert",
		},
		{
			desc:     "module info unavailable",
			from:     "example.com/myproject",
			pkg:      "golang.org/x/text",
			template: `{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{else}}{{.ImportPath}}{{end}}`,
			want:     "golang.org/x/text",
		},
		{
			desc:     "fallback to ImportPath",
			from:     "example.com/myproject",
			pkg:      "unknown.com/package",
			template: `{{.ImportPath}}`,
			want:     "unknown.com/package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			linker.Template(tt.pkg, requireParseTemplate(t, tt.template))
			got := linker.packageDocURL(tt.from, tt.pkg)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDocLinker_versionedLinks(t *testing.T) {
	t.Parallel()

	// Set up a module tree with test dependencies.
	var tree gomod.Tree
	tree.PutModuleDeps("example.com/myproject", []*gomod.Module{
		{Path: "go.uber.org/zap", Version: "v1.27.1"},
		{Path: "github.com/stretchr/testify", Version: "v1.8.4"},
	})

	var linker docLinker
	linker.ModuleTree = &tree
	linker.LocalPackage("example.com/myproject")
	linker.LocalPackage("example.com/myproject/foo")

	tests := []struct {
		desc string
		from string
		link comment.DocLink
		want string
	}{
		{
			desc: "versioned/module root",
			from: "example.com/myproject",
			link: comment.DocLink{
				ImportPath: "go.uber.org/zap",
			},
			want: "https://pkg.go.dev/go.uber.org/zap@v1.27.1",
		},
		{
			desc: "versioned/module subpackage",
			from: "example.com/myproject",
			link: comment.DocLink{
				ImportPath: "go.uber.org/zap/zaptest/observer",
			},
			want: "https://pkg.go.dev/go.uber.org/zap@v1.27.1/zaptest/observer",
		},
		{
			desc: "versioned/with symbol",
			from: "example.com/myproject",
			link: comment.DocLink{
				ImportPath: "go.uber.org/zap",
				Name:       "Logger",
			},
			want: "https://pkg.go.dev/go.uber.org/zap@v1.27.1#Logger",
		},
		{
			desc: "versioned/with method",
			from: "example.com/myproject",
			link: comment.DocLink{
				ImportPath: "github.com/stretchr/testify/assert",
				Recv:       "Assertions",
				Name:       "Equal",
			},
			want: "https://pkg.go.dev/github.com/stretchr/testify@v1.8.4/assert#Assertions.Equal",
		},
		{
			desc: "versioned/from subpackage",
			from: "example.com/myproject/foo",
			link: comment.DocLink{
				ImportPath: "go.uber.org/zap",
			},
			want: "https://pkg.go.dev/go.uber.org/zap@v1.27.1",
		},
		{
			desc: "versioned/unknown dependency falls back",
			from: "example.com/myproject",
			link: comment.DocLink{
				ImportPath: "golang.org/x/text",
			},
			want: "https://pkg.go.dev/golang.org/x/text",
		},
		{
			desc: "versioned/local package not versioned",
			from: "example.com/myproject",
			link: comment.DocLink{
				ImportPath: "example.com/myproject/foo",
			},
			want: "foo",
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
