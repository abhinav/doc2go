// Package html renders HTML from godoc.Package.
package html

import (
	"embed"
	"go/doc/comment"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/relative"
)

const _staticDir = "_"

var (
	//go:embed tmpl/*.html
	_tmplFS embed.FS

	//go:embed static/**
	_staticFS embed.FS

	// Trick borrowed from pkgsite:
	// Unusable function references at parse time,
	// and then Clone and replace at render time.
	// This way, template validity is still
	// verified at init.
	_packageTmpl = template.Must(
		template.New("package.html").
			Funcs((*render)(nil).FuncMap()).
			ParseFS(_tmplFS,
				"tmpl/package.html", "tmpl/layout.html", "tmpl/subpackages.html"),
	)

	_packageIndexTmpl = template.Must(
		template.New("directory.html").
			Funcs((*render)(nil).FuncMap()).
			ParseFS(_tmplFS, "tmpl/directory.html", "tmpl/layout.html", "tmpl/subpackages.html"),
	)
)

// Renderer renders components into HTML.
type Renderer struct {
	// Whether we're in embedded mode.
	// In this mode, output will only contain the documentation output
	// and will not generate complete, stylized HTML pages.
	Embedded bool

	// Internal specifies whether directory listings
	// should include internal packages.
	Internal bool
}

func (r *Renderer) templateName() string {
	if r.Embedded {
		return "Body"
	}
	return "Page"
}

// WriteStatic dumps the contents of static/ into the given directory.
//
// This is a no-op if the renderer is running in embedded mode.
func (r *Renderer) WriteStatic(dir string) error {
	if r.Embedded {
		return nil
	}

	dir = filepath.Join(dir, _staticDir)
	static, err := fs.Sub(_staticFS, "static")
	if err != nil {
		return err
	}
	return fs.WalkDir(static, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "." {
			return err
		}

		outPath := filepath.Join(dir, path)
		if d.IsDir() {
			return os.MkdirAll(outPath, 0o1755)
		}

		bs, err := fs.ReadFile(static, path)
		if err != nil {
			return err
		}

		return os.WriteFile(outPath, bs, 0o644)
	})
}

// Breadcrumb holds information about parents of a page
// so that we can leave a trail up for navigation.
type Breadcrumb struct {
	// Text for the crumb.
	Text string

	// Path to the crumb from the root of the output.
	Path string
}

// PackageInfo specifies the package that should be rendered.
type PackageInfo struct {
	// Parsed package documentation information.
	*godoc.Package

	Subpackages []Subpackage
	Breadcrumbs []Breadcrumb

	// DocPrinter specifies how to render godoc comments.
	DocPrinter DocPrinter
}

// RenderPackage renders the documentation for a single Go package.
// It does not include subpackage information.
func (r *Renderer) RenderPackage(w io.Writer, info *PackageInfo) error {
	render := render{
		Path:       info.ImportPath,
		DocPrinter: info.DocPrinter,
		Internal:   r.Internal,
	}
	return template.Must(_packageTmpl.Clone()).
		Funcs(render.FuncMap()).
		ExecuteTemplate(w, r.templateName(), info)
}

// PackageIndex holds information about a package listing.
type PackageIndex struct {
	// Path to this package index.
	Path string

	Subpackages []Subpackage
	Breadcrumbs []Breadcrumb
}

// Subpackage is a descendant of a Go package.
//
// This is typically a direct descendant,
// but it may be a couple levels deeper
// if there are no intermediate Go packages.
// For example, foo/internal/bar may be a descendant of foo/
// if internal is not a Go package.
type Subpackage struct {
	// RelativePath is the path to the subpackage
	// relative to the package it's a subpackage of.
	RelativePath string

	// Synopsis is a short, one-sentence summary
	// extracted from the package's documentation.
	Synopsis string
}

// RenderPackageIndex renders the list of descendants for a package
// as HTML.
func (r *Renderer) RenderPackageIndex(w io.Writer, pidx *PackageIndex) error {
	render := render{
		Path:     pidx.Path,
		Internal: r.Internal,
	}
	return template.Must(_packageIndexTmpl.Clone()).
		Funcs(render.FuncMap()).
		ExecuteTemplate(w, r.templateName(), pidx)
}

type render struct {
	Path string

	Internal bool

	// DocPrinter converts Go comment.Doc objects into HTML.
	DocPrinter DocPrinter
}

func (r *render) FuncMap() template.FuncMap {
	return template.FuncMap{
		"doc":               r.doc,
		"code":              renderCode,
		"static":            r.static,
		"relativePath":      r.relativePath,
		"filterSubpackages": r.filterSubpackages,
	}
}

func (r *render) relativePath(p string) string {
	return relative.Path(r.Path, p)
}

func (r *render) static(p string) string {
	return r.relativePath(path.Join(_staticDir, p))
}

func (r *render) doc(lvl int, doc *comment.Doc) template.HTML {
	if doc == nil {
		return ""
	}
	return template.HTML(r.DocPrinter.WithHeadingLevel(lvl).HTML(doc))
}

func (r *render) filterSubpackages(pkgs []Subpackage) []Subpackage {
	// No filtering if listing internal packages.
	if r.Internal {
		return pkgs
	}

	filtered := make([]Subpackage, 0, len(pkgs))
	for _, pkg := range pkgs {
		relPath := pkg.RelativePath
		if relPath == "internal" || strings.HasPrefix(relPath, "internal/") {
			continue
		}
		filtered = append(filtered, pkg)
	}
	return filtered
}
