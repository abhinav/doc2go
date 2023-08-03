// Package html renders HTML from godoc.Package.
package html

import (
	"bytes"
	"embed"
	"go/doc/comment"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	ttemplate "text/template"

	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/highlight"
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

// Highlighter renders Go code into HTML.
type Highlighter interface {
	Highlight(*highlight.Code) string
	WriteCSS(io.Writer) error
}

var _ Highlighter = (*highlight.Highlighter)(nil)

// Renderer renders components into HTML.
type Renderer struct {
	// Path to the home page of the generated site.
	Home string

	// Whether we're in embedded mode.
	// In this mode, output will only contain the documentation output
	// and will not generate complete, stylized HTML pages.
	Embedded bool

	// Internal specifies whether directory listings
	// should include internal packages.
	Internal bool

	// FrontMatter to include at the top of each file, if any.
	FrontMatter *ttemplate.Template

	// Highlighter renders code blocks into HTML.
	Highlighter Highlighter

	// NormalizeRelativePath is an optional function that
	// normalizes relative paths printed in the generated HTML.
	NormalizeRelativePath func(string) string
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

		// FIXME: This is a hack. That we need to append to main.css
		// should be represented elsewhere.
		if path == "css/main.css" {
			buff := bytes.NewBuffer(bs)
			buff.WriteString("\n")
			if err := r.Highlighter.WriteCSS(buff); err != nil {
				return err
			}
			bs = buff.Bytes()
		}

		return os.WriteFile(outPath, bs, 0o644)
	})
}

type frontmatterPackageData struct {
	Name     string
	Synopsis string
}

type frontmatterData struct {
	Path        string
	Basename    string
	NumChildren int
	Package     frontmatterPackageData
}

func (r *Renderer) renderFrontmatter(w io.Writer, d frontmatterData) error {
	if r.FrontMatter == nil {
		return nil
	}

	var buff bytes.Buffer
	if err := r.FrontMatter.Execute(&buff, d); err != nil {
		return err
	}

	bs := bytes.TrimSpace(buff.Bytes())
	if len(bs) == 0 {
		return nil
	}
	bs = append(bs, '\n', '\n')

	_, err := w.Write(bs)
	return err
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

	NumChildren int
	Subpackages []Subpackage
	Breadcrumbs []Breadcrumb

	// DocPrinter specifies how to render godoc comments.
	DocPrinter DocPrinter
}

// Basename is the last component of this package's path.
func (b *PackageInfo) Basename() string {
	return filepath.Base(b.ImportPath)
}

// RenderPackage renders the documentation for a single Go package.
// It does not include subpackage information.
func (r *Renderer) RenderPackage(w io.Writer, info *PackageInfo) error {
	err := r.renderFrontmatter(w, frontmatterData{
		Path:        info.ImportPath,
		Basename:    info.Basename(),
		NumChildren: info.NumChildren,
		Package: frontmatterPackageData{
			Name:     info.Name,
			Synopsis: info.Synopsis,
		},
	})
	if err != nil {
		return err
	}
	render := render{
		Home:                  r.Home,
		Path:                  info.ImportPath,
		DocPrinter:            info.DocPrinter,
		Internal:              r.Internal,
		Highlighter:           r.Highlighter,
		NormalizeRelativePath: r.NormalizeRelativePath,
	}
	return template.Must(_packageTmpl.Clone()).
		Funcs(render.FuncMap()).
		ExecuteTemplate(w, r.templateName(), info)
}

// PackageIndex holds information about a package listing.
type PackageIndex struct {
	// Path to this package index.
	Path string

	NumChildren int
	Subpackages []Subpackage
	Breadcrumbs []Breadcrumb
}

// Basename is the last component of this directory's path,
// or if it's the top level directory, an empty string.
func (idx *PackageIndex) Basename() string {
	if len(idx.Path) == 0 {
		return ""
	}
	return filepath.Base(idx.Path)
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
	fmdata := frontmatterData{
		Path:        pidx.Path,
		Basename:    pidx.Basename(),
		NumChildren: pidx.NumChildren,
	}
	if err := r.renderFrontmatter(w, fmdata); err != nil {
		return err
	}
	render := render{
		Home:                  r.Home,
		Path:                  pidx.Path,
		Internal:              r.Internal,
		Highlighter:           r.Highlighter,
		NormalizeRelativePath: r.NormalizeRelativePath,
	}
	return template.Must(_packageIndexTmpl.Clone()).
		Funcs(render.FuncMap()).
		ExecuteTemplate(w, r.templateName(), pidx)
}

type render struct {
	Home string
	Path string

	Internal bool

	// DocPrinter converts Go comment.Doc objects into HTML.
	DocPrinter DocPrinter

	Highlighter           Highlighter
	NormalizeRelativePath func(string) string
}

func (r *render) FuncMap() template.FuncMap {
	return template.FuncMap{
		"doc":               r.doc,
		"code":              r.code,
		"static":            r.static,
		"relativePath":      r.relativePath,
		"filterSubpackages": r.filterSubpackages,
		"normalizeRelativePath": func(p string) string {
			if f := r.NormalizeRelativePath; f != nil {
				return f(p)
			}
			return p
		},
	}
}

func (r *render) relativePath(p string) string {
	p = relative.Path(r.Path, p)
	if r.NormalizeRelativePath != nil {
		p = r.NormalizeRelativePath(p)
	}
	return p
}

func (r *render) static(p string) string {
	return r.relativePath(path.Join(r.Home, _staticDir, p))
}

func (r *render) code(code *highlight.Code) template.HTML {
	return template.HTML(r.Highlighter.Highlight(code))
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
