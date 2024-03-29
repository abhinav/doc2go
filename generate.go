package main

import (
	"context"
	"fmt"
	"go/doc/comment"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"braces.dev/errtrace"
	"go.abhg.dev/doc2go/internal/errdefer"
	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/html"
	"go.abhg.dev/doc2go/internal/pagefind"
	"go.abhg.dev/doc2go/internal/pathtree"
	"go.abhg.dev/doc2go/internal/pathx"
	"go.abhg.dev/doc2go/internal/relative"
	"go.abhg.dev/doc2go/internal/sliceutil"
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
	RenderSiteIndex(io.Writer, *html.SiteIndex) error
}

var _ Renderer = (*html.Renderer)(nil)

// PageIndexer generates a search index for a website.
type PageIndexer interface {
	Index(context.Context, pagefind.IndexRequest) error
}

var _ PageIndexer = (*pagefind.CLI)(nil)

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

	// Pagefind specifies how to generate a search index
	// for the documentation.
	//
	// If nil, a search index will not be generated.
	Pagefind PageIndexer

	DocLinker godoc.Linker

	// OutDir is the destination directory.
	// It will be created it if it doesn't exist.
	OutDir string

	// SubDir is an optional subdirectory inside OutDir.
	// If speciifed, pages will be generated under OutDir/SubDir,
	// and OutDir will get an index of siblings of SubDir.
	//
	// SubDir MUST NOT contain '/'.
	SubDir     string
	PkgVersion string

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
func (r *Generator) Generate(ctx context.Context, pkgRefs []*gosrc.PackageRef) error {
	r.init()

	if err := r.Renderer.WriteStatic(r.OutDir); err != nil {
		return errtrace.Wrap(err)
	}

	trees := buildTrees(pkgRefs)
	if r.Home != "" {
		trees = filterTrees(r.Home, trees)
	}

	if _, err := r.renderTrees(nil, trees); err != nil {
		return errtrace.Wrap(err)
	}

	if r.Pagefind != nil {
		siteDir := filepath.Join(r.OutDir, r.SubDir)
		req := pagefind.IndexRequest{
			SiteDir:     siteDir,
			AssetSubdir: filepath.Join(html.StaticDir, "pagefind"),
		}

		if err := r.Pagefind.Index(ctx, req); err != nil {
			return errtrace.Wrap(fmt.Errorf("generate search index: %w", err))
		}

		r.DebugLog.Printf("Generated search index in %v", req.AssetSubdir)
	}

	if err := r.generateSiblingIndex(); err != nil {
		return errtrace.Wrap(fmt.Errorf("generate version index: %w", err))
	}

	return nil
}

// If a -subdir is specified, generate a listing of siblings
// under the output directory.
// This is useful for generating documentation for multiple versions
// of the same package.
func (r *Generator) generateSiblingIndex() (err error) {
	if r.SubDir == "" {
		return nil
	}

	// "_site/v1.0.0" -> "_site"
	//
	// With the current restriction of SubDir not containing '/',
	// this will always be OutDir,
	// but we're defensively being explicit here for future-proofing.
	siblingDir := filepath.Dir(filepath.Join(r.OutDir, r.SubDir))

	entries, err := os.ReadDir(siblingDir)
	if err != nil {
		return errtrace.Wrap(err)
	}

	idx := html.SiteIndex{Path: r.Home}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == html.StaticDir {
			continue
		}

		idx.Sites = append(idx.Sites, entry.Name())
	}

	f, err := os.Create(filepath.Join(r.OutDir, r.Basename))
	if err != nil {
		return errtrace.Wrap(err)
	}
	defer errdefer.Close(&err, f)

	if err := r.Renderer.RenderSiteIndex(f, &idx); err != nil {
		return errtrace.Wrap(err)
	}

	return nil
}

func (r *Generator) renderTrees(crumbs []html.Breadcrumb, trees []packageTree) ([]*renderedPackage, error) {
	var pkgs []*renderedPackage
	for _, t := range trees {
		rpkgs, err := r.renderTree(crumbs, t)
		if err != nil {
			return nil, errtrace.Wrap(err)
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
	if len(crumbText) > 0 {
		crumbs = append(crumbs, html.Breadcrumb{Text: crumbText, Path: t.Path})
	}

	if t.Value == nil {
		return errtrace.Wrap2(r.renderPackageIndex(crumbs, t))
	}
	rpkg, err := r.renderPackage(crumbs, t)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return []*renderedPackage{rpkg}, nil
}

func (r *Generator) renderPackageIndex(crumbs []html.Breadcrumb, t packageTree) (_ []*renderedPackage, err error) {
	subpkgs, err := r.renderTrees(crumbs, t.Children)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	r.DebugLog.Printf("Rendering directory %v", t.Path)

	dir := filepath.Join(r.OutDir, r.SubDir, relative.Path(r.Home, t.Path))
	if err := os.MkdirAll(dir, 0o1755); err != nil {
		return nil, errtrace.Wrap(err)
	}

	f, err := os.Create(filepath.Join(dir, r.Basename))
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	defer errdefer.Close(&err, f)

	var subdirDepth int
	if r.SubDir != "" {
		subdirDepth = 1 + strings.Count(r.SubDir, "/")
	}

	idx := html.PackageIndex{
		Path:        t.Path,
		SubDirDepth: subdirDepth,
		NumChildren: len(t.Children),
		Subpackages: htmlSubpackages(t.Path, subpkgs),
		Breadcrumbs: crumbs,
	}
	if err := r.Renderer.RenderPackageIndex(f, &idx); err != nil {
		return nil, errtrace.Wrap(err)
	}

	return subpkgs, nil
}

type renderedPackage struct {
	ImportPath string
	Synopsis   string
}

func (r *Generator) renderPackage(crumbs []html.Breadcrumb, t packageTree) (_ *renderedPackage, err error) {
	subpkgs, err := r.renderTrees(crumbs, t.Children)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	ref := *t.Value
	r.DebugLog.Printf("Rendering package %v", t.Path)
	bpkg, err := r.Parser.ParsePackage(ref)
	if err != nil {
		return nil, errtrace.Wrap(fmt.Errorf("parse: %w", err))
	}

	dpkg, err := r.Assembler.Assemble(bpkg)
	if err != nil {
		return nil, errtrace.Wrap(fmt.Errorf("assemble: %w", err))
	}

	dir := filepath.Join(r.OutDir, r.SubDir, relative.Path(r.Home, t.Path))
	if err := os.MkdirAll(dir, 0o1755); err != nil {
		return nil, errtrace.Wrap(err)
	}

	f, err := os.Create(filepath.Join(dir, r.Basename))
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	defer errdefer.Close(&err, f)

	var subdirDepth int
	if r.SubDir != "" {
		subdirDepth = 1 + strings.Count(r.SubDir, "/")
	}

	info := html.PackageInfo{
		Package:     dpkg,
		NumChildren: len(t.Children),
		Breadcrumbs: crumbs,
		Subpackages: htmlSubpackages(dpkg.ImportPath, subpkgs),
		DocPrinter: &html.CommentDocPrinter{
			Printer: comment.Printer{
				DocLinkURL: func(link *comment.DocLink) string {
					return r.DocLinker.DocLinkURL(dpkg.ImportPath, link)
				},
			},
		},
		SubDirDepth: subdirDepth,
		PkgVersion:  r.PkgVersion,
	}
	if err := r.Renderer.RenderPackage(f, &info); err != nil {
		return nil, errtrace.Wrap(fmt.Errorf("render: %w", err))
	}

	return &renderedPackage{
		ImportPath: ref.ImportPath,
		Synopsis:   dpkg.Synopsis,
	}, nil
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
