package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/html"
	"go.abhg.dev/doc2go/internal/iotest"
)

func TestGenerator_hierarchy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc     string
		packages []*fakePackage
		wantPkgs map[string]*renderInfo // import path => info
		wantDirs map[string]*renderInfo // dir path => info
	}{
		{
			desc: "simple",
			packages: []*fakePackage{
				{
					ImportPath: "example.com/foo/bar",
					Synopsis:   "package bar does things.",
				},
			},
			wantPkgs: map[string]*renderInfo{
				"example.com/foo/bar": {
					Breadcrumbs: []html.Breadcrumb{
						{Text: "example.com", Path: "example.com"},
						{Text: "foo", Path: "example.com/foo"},
						{Text: "bar", Path: "example.com/foo/bar"},
					},
				},
			},
			wantDirs: map[string]*renderInfo{
				"example.com/foo": {
					Breadcrumbs: []html.Breadcrumb{
						{Text: "example.com", Path: "example.com"},
						{Text: "foo", Path: "example.com/foo"},
					},
					Subpackages: []html.Subpackage{
						{RelativePath: "bar", Synopsis: "package bar does things."},
					},
				},
				"example.com": {
					Breadcrumbs: []html.Breadcrumb{
						{Text: "example.com", Path: "example.com"},
					},
					Subpackages: []html.Subpackage{
						{RelativePath: "foo/bar", Synopsis: "package bar does things."},
					},
				},
				"": {
					Subpackages: []html.Subpackage{
						{RelativePath: "example.com/foo/bar", Synopsis: "package bar does things."},
					},
				},
			},
		},
		{
			desc: "interlinked",
			packages: []*fakePackage{
				{ImportPath: "a/b/c", Synopsis: "package c"},
				{ImportPath: "a/d", Synopsis: "package d"},
				{ImportPath: "a/b/e", Synopsis: "package e"},
			},
			wantPkgs: map[string]*renderInfo{
				"a/d": {
					Breadcrumbs: []html.Breadcrumb{
						{Text: "a", Path: "a"},
						{Text: "d", Path: "a/d"},
					},
				},
				"a/b/c": {
					Breadcrumbs: []html.Breadcrumb{
						{Text: "a", Path: "a"},
						{Text: "b", Path: "a/b"},
						{Text: "c", Path: "a/b/c"},
					},
				},
				"a/b/e": {
					Breadcrumbs: []html.Breadcrumb{
						{Text: "a", Path: "a"},
						{Text: "b", Path: "a/b"},
						{Text: "e", Path: "a/b/e"},
					},
				},
			},
			wantDirs: map[string]*renderInfo{
				"": {
					Subpackages: []html.Subpackage{
						{RelativePath: "a/b/c", Synopsis: "package c"},
						{RelativePath: "a/b/e", Synopsis: "package e"},
						{RelativePath: "a/d", Synopsis: "package d"},
					},
				},
				"a": {
					Breadcrumbs: []html.Breadcrumb{
						{Text: "a", Path: "a"},
					},
					Subpackages: []html.Subpackage{
						{RelativePath: "b/c", Synopsis: "package c"},
						{RelativePath: "b/e", Synopsis: "package e"},
						{RelativePath: "d", Synopsis: "package d"},
					},
				},
				"a/b": {
					Breadcrumbs: []html.Breadcrumb{
						{Text: "a", Path: "a"},
						{Text: "b", Path: "a/b"},
					},
					Subpackages: []html.Subpackage{
						{RelativePath: "c", Synopsis: "package c"},
						{RelativePath: "e", Synopsis: "package e"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			pkgmap := make(map[string]*fakePackage, len(tt.packages))
			refs := make([]*gosrc.PackageRef, len(tt.packages))
			wantImports := make([]string, len(tt.packages))
			for i, pkg := range tt.packages {
				pkgmap[pkg.ImportPath] = pkg
				wantImports[i] = pkg.ImportPath
				refs[i] = &gosrc.PackageRef{
					Name:       filepath.Base(pkg.ImportPath),
					ImportPath: pkg.ImportPath,
				}
			}

			parser := fakeParser{t: t, packages: pkgmap}
			defer func() {
				assert.ElementsMatch(t, wantImports, parser.sawImports,
					"Parser didn't see all packages")
			}()

			assembler := fakeAssembler{t: t, packages: pkgmap}
			defer func() {
				assert.ElementsMatch(t, wantImports, assembler.sawImports,
					"Assembler didn't see all packages")
			}()

			renderer := fakeRenderer{
				t:               t,
				wantPackages:    tt.wantPkgs,
				wantDirectories: tt.wantDirs,
			}

			g := Generator{
				DebugLog:  log.New(iotest.Writer(t), "", 0),
				Parser:    &parser,
				Assembler: &assembler,
				Renderer:  &renderer,
				OutDir:    t.TempDir(),
			}
			require.NoError(t, g.Generate(refs))
		})
	}
}

func TestGenerator_basename(t *testing.T) {
	pkgs := map[string]*fakePackage{
		"foo": {ImportPath: "foo"},
	}
	parser := fakeParser{
		t:        t,
		packages: pkgs,
	}
	assembler := fakeAssembler{t: t, packages: pkgs}

	renderer := fakeRenderer{
		t: t,
		wantPackages: map[string]*renderInfo{
			"foo": {
				Breadcrumbs: []html.Breadcrumb{
					{Text: "foo", Path: "foo"},
				},
			},
		},
		wantDirectories: map[string]*renderInfo{
			"": {
				Subpackages: []html.Subpackage{
					{RelativePath: "foo"},
				},
			},
		},
	}

	outDir := t.TempDir()
	g := Generator{
		DebugLog:  log.New(iotest.Writer(t), "", 0),
		Parser:    &parser,
		Assembler: &assembler,
		Renderer:  &renderer,
		OutDir:    outDir,
		Basename:  "_index.html",
	}
	require.NoError(t, g.Generate([]*gosrc.PackageRef{
		{
			Name:       "foo",
			ImportPath: "foo",
		},
	}))

	indexPath := filepath.Join(outDir, "foo", "_index.html")
	_, err := os.Stat(indexPath)
	require.NoError(t, err, "file must exist: %v", indexPath)
}

type fakePackage struct {
	ImportPath string
	Synopsis   string
}

type fakeParser struct {
	t          *testing.T
	packages   map[string]*fakePackage // import path => package
	sawImports []string
}

var _ Parser = (*fakeParser)(nil)

func (p *fakeParser) ParsePackage(ref *gosrc.PackageRef) (*gosrc.Package, error) {
	p.sawImports = append(p.sawImports, ref.ImportPath)
	pkg, ok := p.packages[ref.ImportPath]
	require.True(p.t, ok, "unexpected package %q", ref.ImportPath)
	return &gosrc.Package{
		Name:       ref.Name,
		ImportPath: pkg.ImportPath,
	}, nil
}

type fakeAssembler struct {
	t          *testing.T
	packages   map[string]*fakePackage // import path => package
	sawImports []string
}

var _ Assembler = (*fakeAssembler)(nil)

func (as *fakeAssembler) Assemble(bpkg *gosrc.Package) (*godoc.Package, error) {
	as.sawImports = append(as.sawImports, bpkg.ImportPath)
	pkg, ok := as.packages[bpkg.ImportPath]
	require.True(as.t, ok, "unexpected package %q", bpkg.ImportPath)
	return &godoc.Package{
		Name:       bpkg.Name,
		ImportPath: pkg.ImportPath,
		Synopsis:   pkg.Synopsis,
	}, nil
}

type renderInfo struct {
	Breadcrumbs []html.Breadcrumb
	Subpackages []html.Subpackage
}

type fakeRenderer struct {
	t               *testing.T
	wantPackages    map[string]*renderInfo
	wantDirectories map[string]*renderInfo
}

var _ Renderer = (*fakeRenderer)(nil)

func (*fakeRenderer) WriteStatic(string) error { return nil }

func (r *fakeRenderer) RenderPackage(_ io.Writer, pkgInfo *html.PackageInfo) error {
	imppath := pkgInfo.ImportPath
	want, ok := r.wantPackages[imppath]
	require.True(r.t, ok, "unexpected package %q", imppath)
	delete(r.wantPackages, imppath)

	assert.Equal(r.t, want.Breadcrumbs, pkgInfo.Breadcrumbs, "breadcrumbs for %q", imppath)
	assert.Equal(r.t, want.Subpackages, pkgInfo.Subpackages, "subpackages for %q", imppath)
	return nil
}

func (r *fakeRenderer) RenderPackageIndex(_ io.Writer, idx *html.PackageIndex) error {
	path := idx.Path
	want, ok := r.wantDirectories[path]
	require.True(r.t, ok, "unexpected directory %q", path)
	delete(r.wantPackages, path)

	assert.Equal(r.t, want.Breadcrumbs, idx.Breadcrumbs, "breadcrumbs for %q", path)
	assert.Equal(r.t, want.Subpackages, idx.Subpackages, "subpackages for %q", path)
	return nil
}
