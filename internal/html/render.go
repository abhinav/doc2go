// Package html renders HTML from godoc.Package.
package html

import (
	"bytes"
	"embed"
	"fmt"
	"go/doc/comment"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/relative"
)

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
	Embedded bool
}

func (r *Renderer) templateName() string {
	if r.Embedded {
		return "Body"
	}
	return "Page"
}

// DocPrinter formats godoc comments as HTML.
type DocPrinter interface {
	HTML(*comment.Doc) []byte
	WithHeadingLevel(int) DocPrinter
}

// WriteStatic dumps the contents of static/ into the given directory.
func (*Renderer) WriteStatic(dir string) error {
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
		Path: pidx.Path,
	}
	return template.Must(_packageIndexTmpl.Clone()).
		Funcs(render.FuncMap()).
		ExecuteTemplate(w, r.templateName(), pidx)
}

type render struct {
	Path string

	// DocPrinter converts Go comment.Doc objects into HTML.
	DocPrinter DocPrinter
}

func (r *render) FuncMap() template.FuncMap {
	return template.FuncMap{
		"doc":          r.doc,
		"code":         r.code,
		"static":       r.static,
		"relativePath": r.relativePath,
	}
}

func (r *render) relativePath(p string) string {
	return relative.Path(r.Path, p)
}

func (r *render) static(p string) string {
	return r.relativePath(path.Join("_", p))
}

func (r *render) doc(lvl int, doc *comment.Doc) template.HTML {
	return template.HTML(r.DocPrinter.WithHeadingLevel(lvl).HTML(doc))
}

func (*render) code(code *godoc.Code) template.HTML {
	var buf bytes.Buffer
	for _, b := range code.Spans {
		switch b := b.(type) {
		case *godoc.TextSpan:
			template.HTMLEscape(&buf, b.Text)
		case *godoc.CommentSpan:
			buf.WriteString(`<span class="comment">`)
			template.HTMLEscape(&buf, b.Text)
			buf.WriteString(`</span>`)
		case *godoc.AnchorSpan:
			fmt.Fprintf(&buf, "<span id=%q>", b.ID)
			template.HTMLEscape(&buf, b.Text)
			buf.WriteString("</span>")
		case *godoc.LinkSpan:
			fmt.Fprintf(&buf, "<a href=%q>", b.Dest)
			template.HTMLEscape(&buf, b.Text)
			buf.WriteString("</a>")
		case *godoc.ErrorSpan:
			buf.WriteString("<strong>")
			template.HTMLEscape(&buf, []byte(b.Msg))
			buf.WriteString("</strong>")
			buf.WriteString("<pre><code>")
			template.HTMLEscape(&buf, []byte(b.Err.Error()))
			buf.WriteString("</code></pre>")
		default:
			panic(fmt.Sprintf("unrecognized node type %T", b))
		}
	}
	return template.HTML(buf.String())
}
