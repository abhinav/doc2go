package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/iotest"
	"golang.org/x/tools/go/packages/packagestest"
)

func TestMainCmd_help(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		args []string
	}{
		{
			desc: "default help",
			args: []string{"-h"},
		},
		{
			desc: "default help long form",
			args: []string{"--help"},
		},
		{
			desc: "help topic",
			args: []string{"-h=frontmatter"},
		},
		{
			desc: "help topic separate arg",
			args: []string{"-h", "frontmatter"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			exitCode := (&mainCmd{
				Stdout: iotest.Writer(t),
				Stderr: iotest.Writer(t),
			}).Run(tt.args)
			assert.Zero(t, exitCode, "-h should have zero status code")
		})
	}
}

func TestMainCmd_version(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	exitCode := (&mainCmd{
		Stdout: &buff,
		Stderr: iotest.Writer(t),
	}).Run([]string{"-version"})
	assert.Zero(t, exitCode, "-version should have zero status code")

	assert.Contains(t, buff.String(), "doc2go")
	assert.Contains(t, buff.String(), _version)
}

func TestMainCmd_unknownFlag(t *testing.T) {
	t.Parallel()

	exitCode := (&mainCmd{
		Stdout: iotest.Writer(t),
		Stderr: iotest.Writer(t),
	}).Run([]string{"--this-flag-does-not-exist"})
	assert.NotZero(t, exitCode, "unknown flag should have non-zero status code")
}

func TestMainCmd_badTemplate(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	exitCode := (&mainCmd{
		Stdout: iotest.Writer(t),
		Stderr: &buff,
	}).Run([]string{"-pkg-doc", "foo=bar{{.baz", "./..."})
	assert.NotZero(t, exitCode)
	assert.Contains(t, buff.String(), "bad package documentation template")
}

func TestMainCmd_generate(t *testing.T) {
	t.Parallel()

	packagestest.TestAll(t, testMainCmdGenerate)
}

func testMainCmdGenerate(t *testing.T, exporter packagestest.Exporter) {
	tests := []struct {
		desc     string
		flags    []string
		basename string
	}{
		{
			desc:     "default",
			basename: "index.html",
		},
		{
			desc:     "different basename",
			flags:    []string{"-basename", "_index.html"},
			basename: "_index.html",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			exported := packagestest.Export(t, exporter, []packagestest.Module{
				{
					Name: "example.com/foo/bar",
					Files: map[string]any{
						"doc.go": "// Package bar does things.\npackage bar\n",
						"types.go": "package bar\n" +
							"// Bar implements the core logic.\n" +
							"type Bar struct{}",
					},
				},
			})

			outDir := t.TempDir()
			args := append(tt.flags, "-out", outDir, "-debug", "-embed", "./...")

			exitCode := (&mainCmd{
				Stdout:         iotest.Writer(t),
				Stderr:         iotest.Writer(t),
				packagesConfig: exported.Config,
			}).Run(args)
			require.Zero(t, exitCode, "expected success")

			fsys := os.DirFS(outDir)
			gotFiles := make(map[string]string)
			err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}

				got, err := fs.ReadFile(fsys, path)
				if err != nil {
					return err
				}
				gotFiles[path] = string(got)
				t.Logf("Found file %v", path)
				return nil
			})
			require.NoError(t, err)

			getFile := func(p string) (string, bool) {
				body, ok := gotFiles[filepath.Join(filepath.FromSlash(p), tt.basename)]
				return body, ok
			}

			if body, ok := getFile("example.com/foo/bar"); assert.True(t, ok) {
				assert.Contains(t, body, "Package bar does things")
				assert.Contains(t, body, "Bar implements the core logic")
			}

			if body, ok := getFile("example.com/foo"); assert.True(t, ok) {
				assert.Contains(t, body, `href="bar"`)
				assert.Contains(t, body, "Package bar does things")
			}

			if body, ok := getFile("example.com"); assert.True(t, ok) {
				assert.Contains(t, body, ">foo/bar<")
				assert.Contains(t, body, "Package bar does things")
			}
		})
	}
}

func TestMainCmd_frontmatter(t *testing.T) {
	t.Parallel()

	const template = "---\ntitle: {{.Path}}\n---"
	frontmatterFile := filepath.Join(t.TempDir(), "frontmatter.txt")
	require.NoError(t,
		os.WriteFile(frontmatterFile, []byte(template), 0o644))

	exported := packagestest.Export(t,
		packagestest.Modules, []packagestest.Module{
			{
				Name: "foo/bar",
				Files: map[string]any{
					"bar.go": "package bar",
				},
			},
		})

	outDir := t.TempDir()
	assertFilePrefix := func(path, want string) {
		bs, err := os.ReadFile(filepath.Join(outDir, path))
		require.NoError(t, err)

		got := string(bs)
		if !strings.HasPrefix(got, want) {
			t.Errorf("File %v must start with %q\nGot:\n%v", path, want, got)
		}
	}

	exitCode := (&mainCmd{
		Stdout:         iotest.Writer(t),
		Stderr:         iotest.Writer(t),
		packagesConfig: exported.Config,
	}).Run([]string{"-frontmatter", frontmatterFile, "-out", outDir, "./..."})
	require.Zero(t, exitCode, "expected success")

	assertFilePrefix("foo/index.html", "---\ntitle: foo\n---\n\n")
	assertFilePrefix("foo/bar/index.html", "---\ntitle: foo/bar\n---\n\n")
}

func TestMainCmd_frontmatter_errors(t *testing.T) {
	t.Run("bad syntax", func(t *testing.T) {
		fm := filepath.Join(t.TempDir(), "frontmatter.txt")
		require.NoError(t, os.WriteFile(fm, []byte("{{"), 0o644))

		var buff bytes.Buffer
		exitCode := (&mainCmd{
			Stdout: iotest.Writer(t),
			Stderr: &buff,
		}).Run([]string{"-frontmatter", fm, "./..."})
		require.NotZero(t, exitCode, "expected success")
		assert.Contains(t, buff.String(), "bad frontmatter template")
	})

	t.Run("file does not exist", func(t *testing.T) {
		var buff bytes.Buffer
		exitCode := (&mainCmd{
			Stdout: iotest.Writer(t),
			Stderr: &buff,
		}).Run([]string{"-frontmatter", "does-not-exist.txt", "./..."})
		require.NotZero(t, exitCode, "expected success")
		assert.Contains(t, buff.String(), "no such file or directory")
	})
}

func TestMainCmd_home(t *testing.T) {
	t.Parallel()

	exported := packagestest.Export(t,
		packagestest.Modules, []packagestest.Module{
			{
				Name: "foo",
				Files: map[string]any{
					"bar/bar.go":     "package bar",
					"bar/baz/baz.go": "package baz",
					"qux/qux.go":     "package qux",
				},
			},
		})

	outDir := t.TempDir()
	exitCode := (&mainCmd{
		Stdout:         iotest.Writer(t),
		Stderr:         iotest.Writer(t),
		packagesConfig: exported.Config,
	}).Run([]string{"-home", "foo/bar", "-out", outDir, "./..."})
	require.Zero(t, exitCode, "expected success")

	assertFileContains := func(p, c string) {
		bs, err := os.ReadFile(filepath.Join(outDir, p))
		require.NoError(t, err)
		assert.Contains(t, string(bs), c)
	}

	assertFileContains("index.html", "package bar")
	assertFileContains("baz/index.html", "package baz")
}
