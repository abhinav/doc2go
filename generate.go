package main

import (
	"context"
	"fmt"
	"go/doc/comment"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"go.abhg.dev/doc2go/internal/errdefer"
	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/html"
	"go.abhg.dev/doc2go/internal/pathtree"
	"go.abhg.dev/doc2go/internal/pathx"
	"go.abhg.dev/doc2go/internal/relative"
	"go.abhg.dev/doc2go/internal/sliceutil"
	"go.uber.org/cff/scheduler"
)

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
	DebugLog *log.Logger

	// Parser parses package information from PackageRefs.
	Parser Parser

	// Assembler converts parsed Package ASTs
	// into a documentation-level IR.
	Assembler Assembler

	// Renderer renders a package's documentation IR
	// into HTML.
	Renderer Renderer

	DocLinker godoc.Linker

	// OutDir is the destination directory.
	// It will be created it if it doesn't exist.
	OutDir string

	// Basename of generated files.
	//
	// Defaults to index.html.
	Basename string

	// Home page of the documentation.
	// Anything not under this path will be discarded.
	Home string

	once sync.Once
}

func (r *Generator) init() {
	r.once.Do(func() {
		if r.DebugLog == nil {
			r.DebugLog = log.New(io.Discard, "", 0)
		}
		if r.Basename == "" {
			r.Basename = "index.html"
		}
	})
}

// Generate runs the generator over the provided packages.
func (r *Generator) Generate(pkgRefs []*gosrc.PackageRef) error {
	r.init()

	if err := r.Renderer.WriteStatic(r.OutDir); err != nil {
		return err
	}

	trees := buildTrees(pkgRefs)
	if r.Home != "" {
		trees = filterTrees(r.Home, trees)
	}

	sched := (&scheduler.Config{}).New()
	_ = r.renderTrees(sched, nil, trees)
	return sched.Wait(context.Background()) // TODO
}

type future[T any] struct {
	j *scheduler.ScheduledJob
	v func() T
}

func enqueue[T any](
	ctx context.Context,
	sched *scheduler.Scheduler,
	f func(context.Context) (T, error),
) future[T] {
	var v T
	j := sched.Enqueue(ctx, scheduler.Job{
		Run: func(ctx context.Context) (err error) {
			v, err = f(ctx)
			return err
		},
	})
	return future[T]{
		j: j,
		v: func() T { return v },
	}
}

func apply[A, B any](
	ctx context.Context,
	sched *scheduler.Scheduler,
	f future[A],
	g func(A) (B, error),
) future[B] {
	var v B
	j := sched.Enqueue(ctx, scheduler.Job{
		Run: func(ctx context.Context) (err error) {
			v, err = g(f.v())
			return err
		},
		Dependencies: []*scheduler.ScheduledJob{f.j},
	})
	return future[B]{
		j: j,
		v: func() B { return v },
	}
}

func combine[T any](
	ctx context.Context,
	sched *scheduler.Scheduler,
	fs ...future[T],
) future[[]T] {
	var vs []T
	js := make([]*scheduler.ScheduledJob, len(fs))
	for i, f := range fs {
		js[i] = f.j
	}

	j := sched.Enqueue(ctx, scheduler.Job{
		Run: func(ctx context.Context) (err error) {
			res := make([]T, len(fs))
			for i, f := range fs {
				res[i] = f.v()
			}
			vs = res
			return nil
		},
		Dependencies: js,
	})
	return future[[]T]{
		j: j,
		v: func() []T { return vs },
	}
}

func (r *Generator) renderTrees(sched *scheduler.Scheduler, crumbs []html.Breadcrumb, trees []packageTree) future[[]*renderedPackage] {
	var pkgs []future[[]*renderedPackage]
	for _, t := range trees {
		rpkgs := r.renderTree(sched, slices.Clone(crumbs), t)
		pkgs = append(pkgs, rpkgs)
	}

	return apply(context.TODO(), sched, combine(context.TODO(), sched, pkgs...), func(rpkgs [][]*renderedPackage) ([]*renderedPackage, error) {
		result := make([]*renderedPackage, 0, len(rpkgs))
		for _, rpkg := range rpkgs {
			result = append(result, rpkg...)
		}
		return result, nil
	})
}

func (r *Generator) renderTree(sched *scheduler.Scheduler, crumbs []html.Breadcrumb, t packageTree) future[[]*renderedPackage] {
	var crumbText string
	if n := len(crumbs); n > 0 {
		crumbText = relative.Path(crumbs[n-1].Path, t.Path)
	} else {
		crumbText = t.Path
	}
	if len(crumbText) > 0 {
		crumbs = append(crumbs, html.Breadcrumb{Text: crumbText, Path: t.Path})
	}

	if t.Value == nil {
		return r.renderPackageIndex(sched, crumbs, t)
	}
	return apply(context.TODO(), sched, r.renderPackage(sched, crumbs, t), func(pkg *renderedPackage) ([]*renderedPackage, error) {
		return []*renderedPackage{pkg}, nil
	})
}

func (r *Generator) renderPackageIndex(sched *scheduler.Scheduler, crumbs []html.Breadcrumb, t packageTree) future[[]*renderedPackage] {
	subpkgs := r.renderTrees(sched, crumbs, t.Children)
	return apply(context.TODO(), sched, subpkgs, func(rpkgs []*renderedPackage) ([]*renderedPackage, error) {
		r.DebugLog.Printf("Rendering directory %v", t.Path)

		dir := filepath.Join(r.OutDir, relative.Path(r.Home, t.Path))
		if err := os.MkdirAll(dir, 0o1755); err != nil {
			return nil, err
		}

		f, err := os.Create(filepath.Join(dir, r.Basename))
		if err != nil {
			return nil, err
		}
		defer errdefer.Close(&err, f)

		idx := html.PackageIndex{
			Path:        t.Path,
			NumChildren: len(t.Children),
			Subpackages: htmlSubpackages(t.Path, rpkgs),
			Breadcrumbs: crumbs,
		}
		if err := r.Renderer.RenderPackageIndex(f, &idx); err != nil {
			return nil, err
		}

		return rpkgs, nil
	})
}

type renderedPackage struct {
	ImportPath string
	Synopsis   string
}

func (r *Generator) renderPackage(sched *scheduler.Scheduler, crumbs []html.Breadcrumb, t packageTree) future[*renderedPackage] {
	subpkgs := r.renderTrees(sched, crumbs, t.Children)
	return apply(context.TODO(), sched, subpkgs, func(rpkgs []*renderedPackage) (*renderedPackage, error) {
		ref := *t.Value
		r.DebugLog.Printf("Rendering package %v", t.Path)
		bpkg, err := r.Parser.ParsePackage(ref)
		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}

		dpkg, err := r.Assembler.Assemble(bpkg)
		if err != nil {
			return nil, fmt.Errorf("assemble: %w", err)
		}

		dir := filepath.Join(r.OutDir, relative.Path(r.Home, t.Path))
		if err := os.MkdirAll(dir, 0o1755); err != nil {
			return nil, err
		}

		f, err := os.Create(filepath.Join(dir, r.Basename))
		if err != nil {
			return nil, err
		}
		defer errdefer.Close(&err, f)

		info := html.PackageInfo{
			Package:     dpkg,
			NumChildren: len(t.Children),
			Breadcrumbs: crumbs,
			Subpackages: htmlSubpackages(dpkg.ImportPath, rpkgs),
			DocPrinter: &html.CommentDocPrinter{
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
	})
}

func htmlSubpackages(from string, rpkgs []*renderedPackage) []html.Subpackage {
	return sliceutil.Transform(rpkgs, func(rpkg *renderedPackage) html.Subpackage {
		// TODO: track this on packageTree?
		relPath := relative.Path(from, rpkg.ImportPath)

		return html.Subpackage{
			RelativePath: relPath,
			Synopsis:     rpkg.Synopsis,
		}
	})
}

type packageTree = pathtree.Snapshot[*gosrc.PackageRef]

func buildTrees(refs []*gosrc.PackageRef) []packageTree {
	var root pathtree.Root[*gosrc.PackageRef]
	for _, ref := range refs {
		root.Set(ref.ImportPath, ref)
	}
	return []packageTree{{Children: root.Snapshot()}}
}

func filterTrees(root string, is []packageTree) []packageTree {
	var os []packageTree
	for _, i := range is {
		if pathx.Descends(root, i.Path) {
			os = append(os, i)
		} else {
			os = append(os, filterTrees(root, i.Children)...)
		}
	}
	return os
}
