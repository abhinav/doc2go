package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/iotest"
)

func TestFlagHelp(t *testing.T) {
	t.Parallel()

	// Verifies that all registered flags are documented in _defaultHelp.

	_, fset := (&cliParser{Stderr: io.Discard}).newFlagSet(nil)
	fset.VisitAll(func(f *flag.Flag) {
		assert.Contains(t, _defaultHelp, "-"+f.Name)
	})
}

func TestFlagHelp_topics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		topic    string
		contains string
	}{
		{topic: "default", contains: "doc2go"},
		{topic: "frontmatter", contains: "text/template"},
		{topic: "pkg-doc", contains: "documentation"},
		{topic: "highlight", contains: "chroma"},
		{topic: "config", contains: "internal"},
		{topic: "usage", contains: "USAGE"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.topic, func(t *testing.T) {
			t.Parallel()

			var buff bytes.Buffer
			_, err := (&cliParser{
				Stderr: &buff,
			}).Parse([]string{"-h", tt.topic})
			assert.ErrorIs(t, err, errHelp)
			assert.Contains(t, buff.String(), tt.contains)
		})
	}
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
				Config:    "doc2go.rc",
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
				Config:    "doc2go.rc",
				Internal:  true,
				Embed:     true,
				Patterns:  []string{"std", "example.com/..."},
			},
		},
		{
			desc: "basename",
			give: []string{"-basename", "_index.html", "./..."},
			want: params{
				Config:    "doc2go.rc",
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
				Config: "doc2go.rc",
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
				Config:      "doc2go.rc",
				FrontMatter: "fm.txt",
				Patterns:    []string{"./..."},
				OutputDir:   "_site",
			},
		},
		{
			desc: "home",
			give: []string{"-home", "go.abhg.dev/doc2go", "./..."},
			want: params{
				Config:    "doc2go.rc",
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
				Config:              "doc2go.rc",
			},
		},
		{
			desc: "print css",
			give: []string{"-highlight-print-css"},
			want: params{
				Config:            "doc2go.rc",
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

func TestCLIParser_Config(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	t.Run("default file", func(t *testing.T) {
		// Can't run in parallel
		// because of chdir.

		defer func() {
			assert.NoError(t, os.Chdir(wd))
		}()
		dir := t.TempDir()
		require.NoError(t, os.Chdir(dir))

		give := fmt.Sprintf("home example.com\nout %v", dir)
		require.NoError(t,
			os.WriteFile("doc2go.rc", []byte(give), 0o644))

		got, err := (&cliParser{
			Stderr: iotest.Writer(t),
		}).Parse([]string{"./..."})
		require.NoError(t, err)

		assert.Equal(t, &params{
			Home:      "example.com",
			Config:    "doc2go.rc",
			OutputDir: dir,
			Patterns:  []string{"./..."},
		}, got)
	})

	t.Run("custom file", func(t *testing.T) {
		cfgFile := filepath.Join(t.TempDir(), "config")
		give := "embed true\nfrontmatter foo.tmpl\nhighlight tango\n"
		require.NoError(t,
			os.WriteFile(cfgFile, []byte(give), 0o644))

		got, err := (&cliParser{
			Stderr: iotest.Writer(t),
		}).Parse([]string{"-config=" + cfgFile, "./..."})
		require.NoError(t, err)

		assert.Equal(t, &params{
			Embed:       true,
			FrontMatter: "foo.tmpl",
			Highlight:   highlightParams{Theme: "tango"},
			Config:      cfgFile,
			OutputDir:   "_site",
			Patterns:    []string{"./..."},
		}, got)
	})
}

func TestCLIParser_Config_disallowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give string
	}{
		{"highlight-print-css", "highlight tango\nhighlight-print-css\n"},
		{"highlight-list-themes", "home example.com\nhighlight-list-themes\n"},
		{"version", "internal\nversion\n"},
		{"help", "frontmatter foo.tmpl\nhelp\n"},
		{"h", "embed false\nh\n"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			cfgFile := filepath.Join(t.TempDir(), "config")
			require.NoError(t,
				os.WriteFile(cfgFile, []byte(tt.give), 0o644))

			_, err := (&cliParser{
				Stderr: iotest.Writer(t),
			}).Parse([]string{"-config=" + cfgFile, "./..."})
			assert.ErrorContains(t, err, "cannot be set from configuration")
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

func TestConfigFileParser(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		var p *configFileParser
		p.Reject("foo")
		err := p.Parse(strings.NewReader("foo"), func(k, v string) error {
			t.Errorf("unexpected set(%q, %q)", k, v)
			return nil
		})
		require.NoError(t, err)
	})

	var p configFileParser
	p.Reject("version", "list")
	p.Reject("help")

	t.Run("reject", func(t *testing.T) {
		t.Parallel()

		err := p.Parse(strings.NewReader("# foo\nhelp"),
			func(k, v string) error {
				t.Errorf("unexpected set(%q, %q)", k, v)
				return nil
			})
		assert.ErrorContains(t, err, `"help" cannot be set from configuration`)
	})

	t.Run("allow", func(t *testing.T) {
		t.Parallel()

		got := make(map[string]string)
		err := p.Parse(strings.NewReader("foo\nbar baz"),
			func(k, v string) error {
				got[k] = v
				return nil
			})
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"foo": "true",
			"bar": "baz",
		}, got)
	})

	t.Run("set error", func(t *testing.T) {
		t.Parallel()

		giveErr := errors.New("great sadness")
		err := p.Parse(strings.NewReader("foo"),
			func(k, v string) error {
				return giveErr
			})
		assert.ErrorIs(t, err, giveErr)
	})
}

func TestRelLinkStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		give       []string
		want       relLinkStyle
		wantString string
	}{
		{
			desc:       "default",
			want:       relLinkStylePlain,
			wantString: "plain",
		},
		{
			desc:       "plain",
			give:       []string{"-x", "plain"},
			want:       relLinkStylePlain,
			wantString: "plain",
		},
		{
			desc:       "directory",
			give:       []string{"-x", "directory"},
			want:       relLinkStyleDirectory,
			wantString: "directory",
		},
		{
			desc:       "plain/uppercase",
			give:       []string{"-x", "PLAIN"},
			want:       relLinkStylePlain,
			wantString: "plain",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
			fset.SetOutput(iotest.Writer(t))

			var got relLinkStyle
			fset.Var(&got, "x", "")

			require.NoError(t, fset.Parse(tt.give))
			assert.Equal(t, tt.want, got)

			assert.NotPanics(t, func() {
				assert.Equal(t, tt.want, got.Get())
			})

			t.Run("String", func(t *testing.T) {
				assert.Equal(t, tt.wantString, got.String())
			})
		})
	}
}

func TestRelLinkStyle_unrecognized(t *testing.T) {
	t.Parallel()

	t.Run("parse", func(t *testing.T) {
		t.Parallel()

		fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
		fset.SetOutput(iotest.Writer(t))

		var got relLinkStyle
		fset.Var(&got, "x", "")

		err := fset.Parse([]string{"-x", "not-a-style"})
		assert.ErrorContains(t, err, "unrecognized link style")
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "relLinkStyle(42)", relLinkStyle(42).String())
	})
}

func TestRelLinkStyle_Normalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc  string
		style relLinkStyle
		give  string
		want  string
	}{
		{
			desc: "plain/no slash",
			give: "foo",
			want: "foo",
		},
		{
			desc: "plain/slash",
			give: "foo/",
			want: "foo",
		},
		{
			desc:  "directory/no slash",
			style: relLinkStyleDirectory,
			give:  "foo",
			want:  "foo/",
		},
		{
			desc:  "directory/slash",
			style: relLinkStyleDirectory,
			give:  "foo/",
			want:  "foo/",
		},
		{
			desc:  "unknown",
			style: relLinkStyle(42),
			give:  "foo",
			want:  "foo",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.style.Normalize(tt.give))
		})
	}
}
