package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/html"
	"go.abhg.dev/doc2go/internal/pathtree"
)

// Finder searches for packages on-disk based on the provided patterns.
type Finder interface {
	FindPackages(patterns ...string) ([]*gosrc.PackageRef, error)
}

// Parser loads a package reference from disk
// and parses its contents.
type Parser interface {
	ParsePackage(*gosrc.PackageRef) (*gosrc.Package, error)
}

// Assembler consumes a parsed Go source package,
// and builds a documentation representation of it.
type Assembler interface {
	Assemble(*gosrc.Package) (*godoc.Package, error)
}

// Renderer renders a Go package's documentation to HTML.
type Renderer interface {
	RenderPackage(io.Writer, *godoc.Package) error
	RenderSubpackages(io.Writer, []*html.Subpackage) error
}

type Runner struct {
	Log       *log.Logger
	Finder    Finder
	Parser    Parser
	Assembler Assembler
	Renderer  Renderer
	OutDir    string
	LinkTmpl  *templateTree
}

func (r *Runner) Run(patterns []string) error {
	pkgRefs, err := r.Finder.FindPackages(patterns...)
	if err != nil {
		return fmt.Errorf("find packages: %w", err)
	}

	return r.renderTrees(buildTrees(pkgRefs))
}

func (r *Runner) renderTrees(trees []*packageTree) error {
	for _, t := range trees {
		if _, err := r.renderPackageTree(t); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) renderPackageTree(t *packageTree) ([]*renderedPackage, error) {
	if t.Ref == nil {
		return r.renderPackageIndex(t)
	}
	rpkg, err := r.renderPackage(t)
	if err != nil {
		return nil, err
	}
	return []*renderedPackage{rpkg}, nil
}

func (r *Runner) renderPackageIndex(t *packageTree) ([]*renderedPackage, error) {
	// TODO: dedupe
	var subpkgs []*renderedPackage
	for _, child := range t.Children {
		rpkgs, err := r.renderPackageTree(child)
		if err != nil {
			return nil, err
		}
		subpkgs = append(subpkgs, rpkgs...)
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

	if err := r.writePackageIndex(f, t.Path, subpkgs); err != nil {
		return nil, err
	}

	return subpkgs, nil
}

func (r *Runner) writePackageIndex(w io.Writer, from string, rpkgs []*renderedPackage) error {
	var subpkgs []*html.Subpackage
	for _, rpkg := range rpkgs {
		// TODO: track this on packageTree?
		relPath, err := filepath.Rel(from, rpkg.ImportPath)
		if err != nil {
			continue
		}

		subpkgs = append(subpkgs, &html.Subpackage{
			RelativePath: relPath,
			Synopsis:     rpkg.Synopsis,
		})
	}
	return r.Renderer.RenderSubpackages(w, subpkgs)
}

type renderedPackage struct {
	ImportPath string
	Synopsis   string
}

func (r *Runner) renderPackage(t *packageTree) (*renderedPackage, error) {
	var subpkgs []*renderedPackage
	for _, child := range t.Children {
		rpkgs, err := r.renderPackageTree(child)
		if err != nil {
			return nil, err
		}
		subpkgs = append(subpkgs, rpkgs...)
	}

	ref := t.Ref
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

	if err := r.Renderer.RenderPackage(f, dpkg); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	if err := r.writePackageIndex(f, t.Path, subpkgs); err != nil {
		return nil, err
	}

	return &renderedPackage{
		ImportPath: ref.ImportPath,
		Synopsis:   dpkg.Synopsis,
	}, nil
}

type packageTree struct {
	Path     string
	Ref      *gosrc.PackageRef
	Children []*packageTree
}

func buildTrees(refs []*gosrc.PackageRef) []*packageTree {
	var root pathtree.Root[*gosrc.PackageRef]
	for _, ref := range refs {
		root.Set(ref.ImportPath, ref)
	}

	var (
		fromSnaps func([]pathtree.Snapshot[*gosrc.PackageRef]) []*packageTree
		fromSnap  func(pathtree.Snapshot[*gosrc.PackageRef]) *packageTree
	)
	fromSnaps = func(snaps []pathtree.Snapshot[*gosrc.PackageRef]) []*packageTree {
		if len(snaps) == 0 {
			return nil
		}

		trees := make([]*packageTree, len(snaps))
		for i, s := range snaps {
			trees[i] = fromSnap(s)
		}
		return trees
	}
	fromSnap = func(s pathtree.Snapshot[*gosrc.PackageRef]) *packageTree {
		t := packageTree{Path: s.Path}
		if s.Value != nil {
			t.Ref = *s.Value
		}
		t.Children = fromSnaps(s.Children)
		return &t
	}

	return fromSnaps(root.Snapshot())
}
