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
	// TODO: if we do this depth-first, we can get subpackage descriptions
	// for use in the package index for renderPackage.
	for _, t := range trees {
		if t.Ref != nil {
			if err := r.renderPackage(t); err != nil {
				return err
			}
		} else {
			if err := r.renderPackageIndex(t); err != nil {
				return err
			}
		}
		if err := r.renderTrees(t.Children); err != nil {
			return err
		}

	}
	return nil
}

func (r *Runner) renderPackageIndex(t *packageTree) error {
	r.Log.Printf("Rendering index %v", t.Path)

	dir := filepath.Join(r.OutDir, t.Path)
	if err := os.MkdirAll(dir, 0o1755); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	return r.writePackageIndex(f, t.Path, t.Favorites)
}

// TODO: template
func (r *Runner) writePackageIndex(w io.Writer, from string, trees []*packageTree) error {
	if len(trees) == 0 {
		return nil
	}

	subpkgs := make([]*html.Subpackage, len(trees))
	for i, t := range trees {
		relPath, err := filepath.Rel(from, t.Path)
		if err != nil {
			// TODO: Log
			relPath = t.Path
		}

		subpkgs[i] = &html.Subpackage{
			RelativePath: relPath,
			Synopsis:     "", // TODO
		}
	}

	return r.Renderer.RenderSubpackages(w, subpkgs)
}

func (r *Runner) renderPackage(t *packageTree) error {
	ref := t.Ref
	r.Log.Printf("Rendering package %v", t.Path)
	bpkg, err := r.Parser.ParsePackage(ref)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	dpkg, err := r.Assembler.Assemble(bpkg)
	if err != nil {
		return fmt.Errorf("assemble: %w", err)
	}

	dir := filepath.Join(r.OutDir, t.Path)
	if err := os.MkdirAll(dir, 0o1755); err != nil {
		return err
	}

	// TODO: For Hugo, this should be _index.html.
	f, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := r.Renderer.RenderPackage(f, dpkg); err != nil {
		return fmt.Errorf("render: %w", err)
	}

	if err := r.writePackageIndex(f, t.Path, t.Favorites); err != nil {
		return err
	}

	return nil
}

type packageTree struct {
	Path     string
	Ref      *gosrc.PackageRef
	Children []*packageTree

	// Closest descendants of this node
	// that have values attached to them.
	Favorites []*packageTree
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
		for _, c := range t.Children {
			if c.Ref != nil {
				t.Favorites = append(t.Favorites, c)
			} else {
				t.Favorites = append(t.Favorites, c.Favorites...)
			}
		}
		return &t
	}

	return fromSnaps(root.Snapshot())
}
