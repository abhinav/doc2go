package gosrc

import (
	"bytes"
	"io"
	"log"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/iotest"
	"golang.org/x/tools/go/packages/packagestest"
)

func TestFinder(t *testing.T) {
	t.Parallel()

	packagestest.TestAll(t, testFinder)
}

func testFinder(t *testing.T, exporter packagestest.Exporter) {
	// Helper to conditionally set module ref (nil for GOPATH).
	moduleRef := func(exported *packagestest.Exported, exp packagestest.Exporter, path string) *ModuleRef {
		if exp.Name() == "GOPATH" {
			return nil
		}
		// In module mode, packagestest creates a subdirectory for each module.
		// The subdirectory name is the last component of the module path.
		// For example, "example.com/foo" creates a "foo/" subdirectory.
		_, moduleName := filepath.Split(path)
		goMod := filepath.Join(exported.Temp(), moduleName, "go.mod")
		return &ModuleRef{
			Path:  path,
			GoMod: goMod,
		}
	}

	tests := []struct {
		desc     string
		path     string
		files    map[string]any
		tags     []string
		want     func(*packagestest.Exported, packagestest.Exporter) []*PackageRef
		wantMsgs []string // messages printed to stderr
	}{
		{
			desc: "file and its test",
			path: "example.com/foo",
			files: map[string]any{
				"foo.go":      "package foo",
				"foo_test.go": "package foo",
			},
			want: func(exported *packagestest.Exported, exp packagestest.Exporter) []*PackageRef {
				return []*PackageRef{
					{
						Name:       "foo",
						ImportPath: "example.com/foo",
						Module:     moduleRef(exported, exp, "example.com/foo"),
						Files: []string{
							exported.File("example.com/foo", "foo.go"),
						},
						TestFiles: []string{
							exported.File("example.com/foo", "foo_test.go"),
						},
					},
				}
			},
		},
		{
			desc: "skip vendor packages",
			path: "example.com/foo",
			files: map[string]any{
				"foo.go":            "package foo",
				"vendor/bar/baz.go": "package bar",
				"bar/baz.go":        "package bar",
			},
			want: func(exported *packagestest.Exported, exp packagestest.Exporter) []*PackageRef {
				return []*PackageRef{
					{
						Name:       "foo",
						ImportPath: "example.com/foo",
						Module:     moduleRef(exported, exp, "example.com/foo"),
						Files: []string{
							exported.File("example.com/foo", "foo.go"),
						},
					},
					{
						Name:       "bar",
						ImportPath: "example.com/foo/bar",
						Module:     moduleRef(exported, exp, "example.com/foo"),
						Files: []string{
							exported.File("example.com/foo", "bar/baz.go"),
						},
					},
				}
			},
		},
		{
			desc: "build tagged file",
			path: "example.com/bar",
			tags: []string{"mytag"},
			files: map[string]any{
				"bar.go":     "//go:build mytag\n\npackage bar",
				"ignored.go": "//go:build anothertag\n\npackage bar",
			},
			want: func(exported *packagestest.Exported, exp packagestest.Exporter) []*PackageRef {
				return []*PackageRef{
					{
						Name:       "bar",
						ImportPath: "example.com/bar",
						Module:     moduleRef(exported, exp, "example.com/bar"),
						Files: []string{
							exported.File("example.com/bar", "bar.go"),
						},
					},
				}
			},
		},
		{
			desc: "package name base name mismatch",
			path: "example.com/foo-go",
			files: map[string]any{
				"foo.go": "package foo",
			},
			want: func(exported *packagestest.Exported, exp packagestest.Exporter) []*PackageRef {
				return []*PackageRef{
					{
						Name:       "foo",
						ImportPath: "example.com/foo-go",
						Module:     moduleRef(exported, exp, "example.com/foo-go"),
						Files: []string{
							exported.File("example.com/foo-go", "foo.go"),
						},
					},
				}
			},
		},
		{
			desc: "skip package errors",
			path: "example.com/foo",
			files: map[string]any{
				"foo.go":     "package foo",
				"bar/a.go":   "package bar",
				"bar/b.go":   "package", // invalid file
				"baz/baz.go": "package baz",
			},
			want: func(exported *packagestest.Exported, exp packagestest.Exporter) []*PackageRef {
				return []*PackageRef{
					{
						Name:       "foo",
						ImportPath: "example.com/foo",
						Module:     moduleRef(exported, exp, "example.com/foo"),
						Files: []string{
							exported.File("example.com/foo", "foo.go"),
						},
					},
					{
						Name:       "baz",
						ImportPath: "example.com/foo/baz",
						Module:     moduleRef(exported, exp, "example.com/foo"),
						Files: []string{
							exported.File("example.com/foo", "baz/baz.go"),
						},
					},
				}
			},
			wantMsgs: []string{"[example.com/foo/bar]", "b.go:1"},
		},
		{
			desc: "skip only test files",
			path: "example.com/bar",
			files: map[string]any{
				"bar.go":          "package bar",
				"baz/qux_test.go": "package baz",
			},
			want: func(exported *packagestest.Exported, exp packagestest.Exporter) []*PackageRef {
				return []*PackageRef{
					{
						Name:       "bar",
						ImportPath: "example.com/bar",
						Module:     moduleRef(exported, exp, "example.com/bar"),
						Files: []string{
							exported.File("example.com/bar", "bar.go"),
						},
					},
				}
			},
			wantMsgs: []string{"[example.com/bar/baz] No non-test Go files. Skipping."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			exported := packagestest.Export(t, exporter, []packagestest.Module{
				{
					Name:  tt.path,
					Files: tt.files,
				},
			})
			t.Cleanup(exported.Cleanup)

			var buff bytes.Buffer
			f := Finder{
				Tags:           tt.tags,
				Log:            log.New(io.MultiWriter(&buff, iotest.Writer(t)), "", 0),
				DebugLog:       log.New(iotest.Writer(t), "", 0),
				PackagesConfig: exported.Config,
			}

			got, err := f.FindPackages("./...")
			require.NoError(t, err)

			assert.Equal(t, tt.want(exported, exporter), got)
			for _, msg := range tt.wantMsgs {
				assert.Contains(t, buff.String(), msg)
			}
		})
	}
}

func TestFinder_NoPackages(t *testing.T) {
	t.Parallel()

	packagestest.TestAll(t, func(t *testing.T, exporter packagestest.Exporter) {
		exported := packagestest.Export(t, exporter, []packagestest.Module{
			{
				Name: "example.com/foo",
				// no files
			},
		})

		f := Finder{
			Log:            log.New(iotest.Writer(t), "", 0),
			PackagesConfig: exported.Config,
		}
		_, err := f.FindPackages("./...")
		assert.ErrorContains(t, err, "no packages found")
	})
}

func TestFinder_ImportedPackage(t *testing.T) {
	t.Parallel()

	packagestest.TestAll(t, testFinderImportedPackage)
}

func testFinderImportedPackage(t *testing.T, exporter packagestest.Exporter) {
	exported := packagestest.Export(t, exporter, []packagestest.Module{
		{
			Name: "example.com/foo",
			Files: map[string]any{
				"foo.go": "package foo\n" +
					`import "example.com/bar-go"` + "\n" +
					"type Foo = bar.Foo\n",
			},
		},
		{
			Name: "example.com/bar-go",
			Files: map[string]any{
				"bar.go": "package bar\ntype Foo int",
			},
		},
	})

	f := Finder{
		Log:            log.New(iotest.Writer(t), "", 0),
		PackagesConfig: exported.Config,
	}
	refs, err := f.FindPackages("./...")
	require.NoError(t, err)

	// Helper to conditionally set module ref (nil for GOPATH).
	moduleRef := func(exp packagestest.Exporter, path string) *ModuleRef {
		if exp.Name() == "GOPATH" {
			return nil
		}
		// In module mode, packagestest creates a subdirectory for each module.
		// The subdirectory name is the last component of the module path.
		_, moduleName := filepath.Split(path)
		goMod := filepath.Join(exported.Temp(), moduleName, "go.mod")
		return &ModuleRef{
			Path:  path,
			GoMod: goMod,
		}
	}

	assert.Equal(t,
		[]*PackageRef{
			{
				Name:       "foo",
				ImportPath: "example.com/foo",
				Module:     moduleRef(exporter, "example.com/foo"),
				Files: []string{
					exported.File("example.com/foo", "foo.go"),
				},
				Imports: []ImportedPackage{
					{
						Name:       "bar",
						ImportPath: "example.com/bar-go",
					},
				},
			},
		}, refs)
}
