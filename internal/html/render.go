// Package html renders HTML from godoc.Package.
package html

import (
	"bytes"
	"embed"
	"fmt"
	"go/doc/comment"
	"html/template"
	"io"

	"go.abhg.dev/doc2go/internal/code"
	"go.abhg.dev/doc2go/internal/godoc"
)

var (
	//go:embed data/*.html
	_tmplFS embed.FS

	_tmpl = template.Must(
		template.New("").
			Funcs(template.FuncMap{
				// Trick borrowed from pkgsite:
				// Unusable function references at parse time,
				// and then Clone and replace at render time.
				// This way, template validity is still
				// verified at init.
				"doc":  (*packageRenderer).doc,
				"code": (*packageRenderer).code,
			}).
			ParseFS(_tmplFS, "data/*"),
	)
)

// Renderer renders components into HTML.
type Renderer struct{}

// DocPrinter formats godoc comments as HTML.
type DocPrinter interface {
	HTML(*comment.Doc) []byte
}

var _ DocPrinter = (*comment.Printer)(nil)

// PackageInfo specifies the package that should be rendered.
type PackageInfo struct {
	// Parsed package documentation information.
	Package *godoc.Package

	// DocPrinter specifies how to render godoc comments.
	DocPrinter DocPrinter
}

// RenderPackage renders the documentation for a single Go package.
// It does not include subpackage information.
func (*Renderer) RenderPackage(w io.Writer, info *PackageInfo) error {
	pkgRender := packageRenderer{
		DocPrinter: info.DocPrinter,
	}

	return template.Must(_tmpl.Clone()).Funcs(template.FuncMap{
		"doc":  pkgRender.doc,
		"code": pkgRender.code,
	}).ExecuteTemplate(w, "package.html", info.Package)
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

// RenderSubpackages renders the list of descendants for a package
// as HTML.
func (*Renderer) RenderSubpackages(w io.Writer, pkgs []*Subpackage) error {
	return template.Must(_tmpl.Clone()).ExecuteTemplate(w, "subpackages.html", struct {
		Subpackages []*Subpackage
	}{Subpackages: pkgs})
}

type packageRenderer struct {
	// DocPrinter converts Go comment.Doc objects into HTML.
	DocPrinter DocPrinter
}

func (r *packageRenderer) doc(doc *comment.Doc) template.HTML {
	return template.HTML(r.DocPrinter.HTML(doc))
}

func (*packageRenderer) code(blk *code.Block) template.HTML {
	var buf bytes.Buffer
	for _, b := range blk.Nodes {
		switch b := b.(type) {
		case *code.TextNode:
			template.HTMLEscape(&buf, b.Text)
		case *code.AnchorNode:
			fmt.Fprintf(&buf, "<a id=%q>", b.ID)
			template.HTMLEscape(&buf, b.Text)
			buf.WriteString("</a>")
		case *code.LinkNode:
			fmt.Fprintf(&buf, "<a href=%q>", b.Dest)
			template.HTMLEscape(&buf, b.Text)
			buf.WriteString("</a>")
		default:
			panic(fmt.Sprintf("unrecognized node type %T", b))
		}
	}
	return template.HTML(buf.String())
}
