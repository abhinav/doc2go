package main

import (
	"bytes"
	"flag"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/iotest"
)

func TestFlagHelp(t *testing.T) {
	// Verifies that all registered flags are documented in _defaultHelp.

	_, fset := (&cliParser{Stderr: io.Discard}).newFlagSet()
	fset.VisitAll(func(f *flag.Flag) {
		assert.Contains(t, _defaultHelp, "-"+f.Name)
	})
}

func TestCLIParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give []string
		want params
	}{
		{
			desc: "minimal",
			give: []string{"./..."},
			want: params{
				OutputDir: "_site",
				Patterns:  []string{"./..."},
			},
		},
		{
			desc: "many arguments",
			give: []string{
				"-tags", "foo,bar",
				"-debug=log.txt",
				"-out", "build/site",
				"-internal",
				"-embed",
				"std",
				"example.com/...",
			},
			want: params{
				Tags:      "foo,bar",
				Debug:     "log.txt",
				OutputDir: "build/site",
				Internal:  true,
				Embed:     true,
				Patterns:  []string{"std", "example.com/..."},
			},
		},
		{
			desc: "basename",
			give: []string{"-basename", "_index.html", "./..."},
			want: params{
				OutputDir: "_site",
				Basename:  "_index.html",
				Patterns:  []string{"./..."},
			},
		},
		{
			desc: "package doc templates",
			give: []string{
				"-pkg-doc", "example.com/foo=https://godocs.io/{{.ImportPath}}",
				"-pkg-doc=example.com/bar=https://go.example.com/{{.ImportPath}}",
				"./...",
			},
			want: params{
				PkgDocs: []pathTemplate{
					{
						Path:     "example.com/foo",
						Template: "https://godocs.io/{{.ImportPath}}",
					},
					{
						Path:     "example.com/bar",
						Template: "https://go.example.com/{{.ImportPath}}",
					},
				},
				Patterns:  []string{"./..."},
				OutputDir: "_site",
			},
		},
		{
			desc: "frontmatter",
			give: []string{
				"-frontmatter", "fm.txt",
				"./...",
			},
			want: params{
				FrontMatter: "fm.txt",
				Patterns:    []string{"./..."},
				OutputDir:   "_site",
			},
		},
		{
			desc: "home",
			give: []string{"-home", "go.abhg.dev/doc2go", "./..."},
			want: params{
				Home:      "go.abhg.dev/doc2go",
				Patterns:  []string{"./..."},
				OutputDir: "_site",
			},
		},
		{
			desc: "list themes",
			give: []string{"-highlight-list-themes"},
			want: params{
				HighlightListThemes: true,
				Patterns:            []string{},
				OutputDir:           "_site",
			},
		},
		{
			desc: "print css",
			give: []string{"-highlight-print-css"},
			want: params{
				HighlightPrintCSS: true,
				Patterns:          []string{},
				OutputDir:         "_site",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got, err := (&cliParser{
				Stderr: iotest.Writer(t),
			}).Parse(tt.give)
			require.NoError(t, err)
			assert.Equal(t, tt.want, *got)
		})
	}
}

func TestCLIParser_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give []string
		want string // expected messages
	}{
		{
			desc: "no patterns",
			want: "Please provide at least one pattern",
		},
		{
			desc: "unrecognized",
			give: []string{"-foo=bar", "./..."},
			want: "flag provided but not defined: -foo",
		},
		{
			desc: "missing '=' in template",
			give: []string{"-pkg-doc", "foo"},
			want: "expected form 'path=template'",
		},
		{
			desc: "bad highlight mode",
			give: []string{"-highlight", "foo:bar"},
			want: `unrecognized highlight mode "foo"`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			_, err := (&cliParser{Stderr: &stderr}).Parse(tt.give)
			require.Error(t, err)
			assert.Contains(t, stderr.String(), tt.want)
		})
	}
}

func TestHighlightParams_Parse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		give       []string
		want       highlightParams
		wantString string
	}{
		{
			desc:       "default",
			give:       []string{"-x", ""},
			want:       highlightParams{Mode: highlightModeAuto},
			wantString: "auto:",
		},
		{
			desc:       "explicit auto",
			give:       []string{"-x", "auto:"},
			want:       highlightParams{Mode: highlightModeAuto},
			wantString: "auto:",
		},
		{
			desc:       "theme only",
			give:       []string{"-x", "foo"},
			want:       highlightParams{Theme: "foo", Mode: highlightModeAuto},
			wantString: "auto:foo",
		},
		{
			desc:       "mode only",
			give:       []string{"-x", "classes:"},
			want:       highlightParams{Mode: highlightModeClasses},
			wantString: "classes:",
		},
		{
			desc:       "mode and theme",
			give:       []string{"-x", "inline:foo"},
			want:       highlightParams{Theme: "foo", Mode: highlightModeInline},
			wantString: "inline:foo",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
			fset.SetOutput(iotest.Writer(t))

			var got highlightParams
			fset.Var(&got, "x", "")
			require.NoError(t, fset.Parse(tt.give))
			assert.Equal(t, tt.want, got)

			assert.NotPanics(t, func() {
				_ = got.Get()
			})

			t.Run("String", func(t *testing.T) {
				assert.Equal(t, tt.wantString, got.String())
			})
		})
	}
}

func TestPathTemplate(t *testing.T) {
	t.Parallel()

	fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
	fset.SetOutput(iotest.Writer(t))

	var pt pathTemplate
	fset.Var(&pt, "x", "")
	require.NoError(t, fset.Parse([]string{
		"-x", "foo=bar",
	}))

	assert.Equal(t, "foo", pt.Path)
	assert.Equal(t, "bar", pt.Template)
	assert.NotNil(t, pt.Template)

	assert.NotNil(t, pt.Get(), "Get")
	assert.Equal(t, "foo=bar", pt.String())
}
