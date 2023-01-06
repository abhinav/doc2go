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
	// Verifies that all registered flags are documented in flags.txt.

	_, fset := (&cliParser{Stderr: io.Discard}).newFlagSet()
	fset.VisitAll(func(f *flag.Flag) {
		assert.Contains(t, _flagDefaults, "-"+f.Name)
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
