package main

import (
	"fmt"
	"go/doc/comment"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/html"
	"go.abhg.dev/doc2go/internal/pathtree"
	"go.abhg.dev/doc2go/internal/relative"
)

// Finder searches for packages on-disk based on the provided patterns.
type Finder interface {
	FindPackages(patterns ...string) ([]*gosrc.PackageRef, error)
}

var _ Finder = (*gosrc.Finder)(nil)

// Parser loads a package reference from disk
// and parses its contents.
type Parser interface {
	ParsePackage(*gosrc.PackageRef) (*gosrc.Package, error)
}

var _ Parser = (*gosrc.Parser)(nil)

// Assembler consumes a parsed Go source package,
// and builds a documentation representation of it.
type Assembler interface {
	Assemble(*gosrc.Package) (*godoc.Package, error)
}

var _ Assembler = (*godoc.Assembler)(nil)

// Renderer renders a Go package's documentation to HTML.
type Renderer interface {
	WriteStatic(string) error
	RenderPackage(io.Writer, *html.PackageInfo) error
	RenderPackageIndex(io.Writer, *html.PackageIndex) error
}

var _ Renderer = (*html.Renderer)(nil)

// Generator generates documentation for user-specified Go packages.
//
// In terms of code organization,
// Generator's purpose is to add a separation between main
// and the program's core logic to aid in testability.
type Generator struct {
	Log       *log.Logger
	Finder    Finder
	Parser    Parser
	Assembler Assembler
	Renderer  Renderer
	OutDir    string
	Internal  bool
	DocLinker godoc.Linker
}

// Generate runs the generator over the provided packages.
func (r *Generator) Generate(pkgRefs []*gosrc.PackageRef) error {
	if err := r.Renderer.WriteStatic(r.OutDir); err != nil {
		return err
	}

	_, err := r.renderTrees(nil, buildTrees(pkgRefs))
	return err
}

func (r *Generator) renderTrees(crumbs []html.Breadcrumb, trees []packageTree) ([]*renderedPackage, error) {
	var pkgs []*renderedPackage
	for _, t := range trees {
		rpkgs, err := r.renderTree(crumbs, t)
		if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, rpkgs...)
	}
	return pkgs, nil
}

func (r *Generator) renderTree(crumbs []html.Breadcrumb, t packageTree) ([]*renderedPackage, error) {
	var crumbText string
	if n := len(crumbs); n > 0 {
		crumbText = relative.Path(crumbs[n-1].Path, t.Path)
	} else {
		crumbText = t.Path
	}
	crumbs = append(crumbs, html.Breadcrumb{Text: crumbText, Path: t.Path})

	if t.Value == nil {
		return r.renderPackageIndex(crumbs, t)
	}
	rpkg, err := r.renderPackage(crumbs, t)
	if err != nil {
		return nil, err
	}
	return []*renderedPackage{rpkg}, nil
}

func (r *Generator) renderPackageIndex(crumbs []html.Breadcrumb, t packageTree) ([]*renderedPackage, error) {
	subpkgs, err := r.renderTrees(crumbs, t.Children)
	if err != nil {
		return nil, err
	}

	r.Log.Printf("Rendering index %v", t.Path)

	dir := filepath.Join(r.OutDir, t.Path)
	if err := os.MkdirAll(dir, 0o1755); err != nil {
		return nil, err
	}

	f, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	idx := html.PackageIndex{
		Path:        t.Path,
		Subpackages: r.subpackages(t.Path, subpkgs),
		Breadcrumbs: crumbs,
	}
	if err := r.Renderer.RenderPackageIndex(f, &idx); err != nil {
		return nil, err
	}

	return subpkgs, nil
}

type renderedPackage struct {
	ImportPath string
	Synopsis   string
}

func (r *Generator) renderPackage(crumbs []html.Breadcrumb, t packageTree) (*renderedPackage, error) {
	subpkgs, err := r.renderTrees(crumbs, t.Children)
	if err != nil {
		return nil, err
	}

	ref := *t.Value
	r.Log.Printf("Rendering package %v", t.Path)
	bpkg, err := r.Parser.ParsePackage(ref)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	dpkg, err := r.Assembler.Assemble(bpkg)
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}

	dir := filepath.Join(r.OutDir, t.Path)
	if err := os.MkdirAll(dir, 0o1755); err != nil {
		return nil, err
	}

	// TODO: For Hugo, this should be _index.html.
	f, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info := html.PackageInfo{
		Package:     dpkg,
		Breadcrumbs: crumbs,
		Subpackages: r.subpackages(dpkg.ImportPath, subpkgs),
		DocPrinter: &docPrinter{
			Printer: comment.Printer{
				DocLinkURL: func(link *comment.DocLink) string {
					return r.DocLinker.DocLinkURL(dpkg.ImportPath, link)
				},
			},
		},
	}
	if err := r.Renderer.RenderPackage(f, &info); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	return &renderedPackage{
		ImportPath: ref.ImportPath,
		Synopsis:   dpkg.Synopsis,
	}, nil
}

func (r *Generator) subpackages(from string, rpkgs []*renderedPackage) []html.Subpackage {
	subpkgs := make([]html.Subpackage, 0, len(rpkgs))
	for _, rpkg := range rpkgs {
		// TODO: track this on packageTree?
		relPath := relative.Filepath(from, rpkg.ImportPath)

		if relPath == "internal" || strings.HasPrefix(relPath, "internal/") {
			if !r.Internal {
				continue
			}
		}

		subpkgs = append(subpkgs, html.Subpackage{
			RelativePath: relPath,
			Synopsis:     rpkg.Synopsis,
		})
	}
	return subpkgs
}

type packageTree = pathtree.Snapshot[*gosrc.PackageRef]

func buildTrees(refs []*gosrc.PackageRef) []packageTree {
	var root pathtree.Root[*gosrc.PackageRef]
	for _, ref := range refs {
		root.Set(ref.ImportPath, ref)
	}
	return root.Snapshot()
}

type docPrinter struct{ comment.Printer }

func (dp *docPrinter) WithHeadingLevel(lvl int) html.DocPrinter {
	out := *dp
	out.HeadingLevel = lvl
	return &out
}
