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

	exitCode := (&mainCmd{
		Stdout: iotest.Writer(t),
		Stderr: iotest.Writer(t),
	}).Run([]string{"-h"})
	assert.Zero(t, exitCode, "-h should have zero status code")
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

	mod := packagestest.Module{
		Name: "foo/bar",
		Files: map[string]any{
			"bar.go": "package bar",
		},
	}
	assertFileHasPrefix := func(t *testing.T, path, want string) {
		bs, err := os.ReadFile(path)
		require.NoError(t, err)

		got := string(bs)
		if !strings.HasPrefix(got, want) {
			t.Errorf("File %v must start with %q\nGot:\n%v", path, want, got)
		}
	}

	tests := []struct {
		desc    string
		give    []string
		wantFoo string
		wantBar string
	}{
		{
			desc: "frontmatter flag",
			give: []string{
				"-frontmatter", template,
			},
			wantFoo: "---\ntitle: foo\n---\n\n",
			wantBar: "---\ntitle: foo/bar\n---\n\n",
		},
		{
			desc: "frontmatter file",
			give: []string{
				"-frontmatter-file", frontmatterFile,
			},
			wantFoo: "---\ntitle: foo\n---\n\n",
			wantBar: "---\ntitle: foo/bar\n---\n\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			exported := packagestest.Export(t,
				packagestest.Modules, []packagestest.Module{mod})

			outDir := t.TempDir()
			args := append(tt.give, "-out", outDir, "-debug", "./...")

			exitCode := (&mainCmd{
				Stdout:         iotest.Writer(t),
				Stderr:         iotest.Writer(t),
				packagesConfig: exported.Config,
			}).Run(args)
			require.Zero(t, exitCode, "expected success")

			assertFileHasPrefix(t,
				filepath.Join(outDir, "foo/index.html"), tt.wantFoo)
			assertFileHasPrefix(t,
				filepath.Join(outDir, "foo/bar/index.html"), tt.wantBar)
		})
	}
}

func TestMainCmd_frontmatter_errors(t *testing.T) {
	t.Run("bad syntax", func(t *testing.T) {
		var buff bytes.Buffer
		exitCode := (&mainCmd{
			Stdout: iotest.Writer(t),
			Stderr: &buff,
		}).Run([]string{"-frontmatter", "{{", "./..."})
		require.NotZero(t, exitCode, "expected success")
		assert.Contains(t, buff.String(), "bad frontmatter template")
	})

	t.Run("file does not exist", func(t *testing.T) {
		var buff bytes.Buffer
		exitCode := (&mainCmd{
			Stdout: iotest.Writer(t),
			Stderr: &buff,
		}).Run([]string{"-frontmatter-file", "does-not-exist.txt", "./..."})
		require.NotZero(t, exitCode, "expected success")
		assert.Contains(t, buff.String(), "no such file or directory")
	})
}
