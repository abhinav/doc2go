package main

import (
	"bytes"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/iotest"
)

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
				Embedded:  true,
				Patterns:  []string{"std", "example.com/..."},
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

	t.Run("package doc templates", func(t *testing.T) {
		got, err := (&cliParser{
			Stderr: iotest.Writer(t),
		}).Parse([]string{
			"-pkg-doc", "example.com/foo=https://godocs.io/{{.ImportPath}}",
			"-pkg-doc=example.com/bar=https://go.example.com/{{.ImportPath}}",
			"./...",
		})
		require.NoError(t, err)

		tmpls := got.PackageDocTemplates
		require.Len(t, tmpls, 2)

		assert.Equal(t, "example.com/foo", tmpls[0].Path)
		assert.Equal(t, "https://godocs.io/{{.ImportPath}}", tmpls[0].rawTmpl)

		assert.Equal(t, "example.com/bar", tmpls[1].Path)
		assert.Equal(t, "https://go.example.com/{{.ImportPath}}", tmpls[1].rawTmpl)
	})
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
	assert.Equal(t, "bar", pt.rawTmpl)
	assert.NotNil(t, pt.Template)

	assert.NotNil(t, pt.Get(), "Get")
	assert.Equal(t, "foo=bar", pt.String())
}

func TestPathTemplate_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give string
		want string // expected error
	}{
		{
			desc: "no '='",
			give: "foo",
			want: "expected form 'path=template'",
		},
		{
			desc: "invalid template",
			give: "foo=bar{{.baz",
			want: "bad template",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
			fset.SetOutput(iotest.Writer(t))

			fset.Var(new(pathTemplate), "x", "")
			err := fset.Parse([]string{"-x", tt.give})
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
