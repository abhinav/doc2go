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
	"strings"
	ttemplate "text/template"

	"braces.dev/errtrace"
	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/highlight"
	"go.abhg.dev/doc2go/internal/relative"
)

// StaticDir is the name of the directory in the output
// where static files are stored.
const StaticDir = "_"

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
				"tmpl/package.html", "tmpl/layout.html", "tmpl/subpackages.html", "tmpl/pagefind.html"),
	)

	_packageIndexTmpl = template.Must(
		template.New("directory.html").
			Funcs((*render)(nil).FuncMap()).
			ParseFS(_tmplFS,
				"tmpl/directory.html", "tmpl/layout.html", "tmpl/subpackages.html", "tmpl/pagefind.html"),
	)

	_siteIndexTmpl = template.Must(
		template.New("siteindex.html").
			Funcs((*render)(nil).FuncMap()).
			ParseFS(_tmplFS,
				"tmpl/siteindex.html", "tmpl/layout.html", "tmpl/pagefind.html"),
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

	// Pagefind specifies whether we have enabled client-side search with
	// pagefind.
	Pagefind bool
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

	dir = filepath.Join(dir, StaticDir)
	static, err := fs.Sub(_staticFS, "static")
	if err != nil {
		return errtrace.Wrap(err)
	}
	return errtrace.Wrap(fs.WalkDir(static, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "." {
			return errtrace.Wrap(err)
		}

		outPath := filepath.Join(dir, path)
		if d.IsDir() {
			return errtrace.Wrap(os.MkdirAll(outPath, 0o1755))
		}

		bs, err := fs.ReadFile(static, path)
		if err != nil {
			return errtrace.Wrap(err)
		}

		// FIXME: This is a hack. That we need to append to main.css
		// should be represented elsewhere.
		if path == "css/main.css" {
			buff := bytes.NewBuffer(bs)
			buff.WriteString("\n")
			if err := r.Highlighter.WriteCSS(buff); err != nil {
				return errtrace.Wrap(err)
			}
			bs = buff.Bytes()
		}

		return errtrace.Wrap(os.WriteFile(outPath, bs, 0o644))
	}))
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

func (d frontmatterData) Name() string {
	if n := d.Package.Name; n != "" && n != "main" {
		return n
	}
	if d.Basename != "" {
		return d.Basename
	}
	return ""
}

func (r *Renderer) renderFrontmatter(w io.Writer, d frontmatterData) error {
	if r.FrontMatter == nil {
		return nil
	}

	var buff bytes.Buffer
	if err := r.FrontMatter.Execute(&buff, d); err != nil {
		return errtrace.Wrap(err)
	}

	bs := bytes.TrimSpace(buff.Bytes())
	if len(bs) == 0 {
		return nil
	}
	bs = append(bs, '\n', '\n')

	_, err := w.Write(bs)
	return errtrace.Wrap(err)
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

	SubDirDepth int
	PkgVersion  string

	// DocPrinter specifies how to render godoc comments.
	DocPrinter DocPrinter
}

// Basename is the last component of this package's path.
func (b *PackageInfo) Basename() string {
	return filepath.Base(b.ImportPath)
}

// IsInternal reports whether this package should be considered
// internal to some other package.
func (b *PackageInfo) IsInternal() bool {
	return isInternal(b.ImportPath)
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
		return errtrace.Wrap(err)
	}
	render := render{
		Home:                  r.Home,
		Path:                  info.ImportPath,
		DocPrinter:            info.DocPrinter,
		Internal:              r.Internal,
		Highlighter:           r.Highlighter,
		NormalizeRelativePath: r.NormalizeRelativePath,
		SubDirDepth:           info.SubDirDepth,
		Pagefind:              r.Pagefind,
	}
	return errtrace.Wrap(template.Must(_packageTmpl.Clone()).
		Funcs(render.FuncMap()).
		ExecuteTemplate(w, r.templateName(), info))
}

// PackageIndex holds information about a package listing.
type PackageIndex struct {
	// Path to this package index.
	Path string

	// Number of levels under output directory
	// that this package index is being generated for.
	//
	// 0 means it's being written to the output directory.
	// 1 means it's being written to a subdirectory of the output directory.
	SubDirDepth int

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

// IsInternal reports whether packages under this index
// should be considered internal to some other package.
func (idx *PackageIndex) IsInternal() bool {
	return isInternal(idx.Path)
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
		return errtrace.Wrap(err)
	}
	render := render{
		Home:                  r.Home,
		Path:                  pidx.Path,
		SubDirDepth:           pidx.SubDirDepth,
		Internal:              r.Internal,
		Highlighter:           r.Highlighter,
		NormalizeRelativePath: r.NormalizeRelativePath,
		Pagefind:              r.Pagefind,
	}
	return errtrace.Wrap(template.Must(_packageIndexTmpl.Clone()).
		Funcs(render.FuncMap()).
		ExecuteTemplate(w, r.templateName(), pidx))
}

// SiteIndex holds information about the root-level site list.
// It's used when the -subdir flag is used to generate
// the top-level index of the various sub-sites.
type SiteIndex struct {
	// Path will be empty unless -home was used.
	Path string

	Sites []string
}

// RenderSiteIndex renders the list of sub-sites as HTML.
func (r *Renderer) RenderSiteIndex(w io.Writer, sidx *SiteIndex) error {
	var data struct {
		*SiteIndex

		Breadcrumbs []Breadcrumb // unused
	}

	data.SiteIndex = sidx

	render := render{
		Home:                  r.Home,
		Path:                  sidx.Path,
		NormalizeRelativePath: r.NormalizeRelativePath,
	}
	return errtrace.Wrap(template.Must(_siteIndexTmpl.Clone()).
		Funcs(render.FuncMap()).
		ExecuteTemplate(w, r.templateName(), data))
}

type render struct {
	Home string
	Path string

	SubDirDepth int
	Internal    bool
	Pagefind    bool

	// DocPrinter converts Go comment.Doc objects into HTML.
	DocPrinter DocPrinter

	Highlighter           Highlighter
	NormalizeRelativePath func(string) string
}

func (r *render) FuncMap() template.FuncMap {
	// NOTE:
	// This function cannot have any state that relies on reading from the
	// render struct because it's called at init time with a nil receiver.
	return template.FuncMap{
		"doc":      r.doc,
		"code":     r.code,
		"pagefind": func() bool { return r.Pagefind },
		// pagefindIgnore:
		// Helpers to add the "data-pagefind-ignore" tag.
		// No-op if pagefind is disabled.
		"pagefindIgnore": func() template.HTMLAttr {
			if !r.Pagefind {
				return ""
			}
			// Extra space because this will be next to a tag.
			return " data-pagefind-ignore"
		},
		"static":     r.static,
		"siteStatic": r.siteStatic,
		// relativevPath:
		// Returns the relative path to the package or directory
		// identified by the given import path.
		// Adds a trailing '/' or not as requested by the user.
		"relativePath": r.relativePath,
		// outputRootRelative:
		// The relative path to the root of the output directory.
		// Includes a trailing '/' if requested by the user.
		"outputRootRelative": func() string {
			root := r.Home
			if r.SubDirDepth > 0 {
				root = path.Join(root, strings.Repeat("../", r.SubDirDepth))
			}
			return r.relativePath(root)
		},
		// siteRootRelative:
		// The relative path to the root of the site directory.
		// Includes a trailing '/' if requested by the user.
		// This is the same as outputRootRelative if -subdir was not used.
		"siteRootRelative": func() string {
			return r.relativePath(r.Home)
		},
		"filterSubpackages": r.filterSubpackages,
		// normalizeRelativePath:
		// Normalizes a relative path to have a '/' or not
		// depending on the rel-link-style flag.
		"normalizeRelativePath": func(p string) string {
			if f := r.NormalizeRelativePath; f != nil {
				return f(p)
			}
			return p
		},
		// dict(k1, v1, k2, v2, ...):
		// Turns key-value pairs into a map.
		// Useful for building objects in templates.
		"dict": dict,
	}
}

// Returns the relative path to the package or directory
// identified by the given import path,
// based on the package being generated.
//
// Adds a trailing '/' or not as requested by the user.
func (r *render) relativePath(p string) string {
	p = relative.Path(r.Path, p)
	if r.NormalizeRelativePath != nil {
		p = r.NormalizeRelativePath(p)
	}
	return p
}

// Returns the path to a static asset stored in the output directory.
// If -subdir was used, these assets are shared with other websites.
func (r *render) static(p string) string {
	elem := []string{r.Home}
	for i := 0; i < r.SubDirDepth; i++ {
		elem = append(elem, "..")
	}
	elem = append(elem, StaticDir, p)
	return r.relativePath(path.Join(elem...))
}

// Returns the path to an asset stored in the site directory
// (outdir/subdir/static).
// This is the same as static if -subdir was not used.
func (r *render) siteStatic(p string) string {
	elem := []string{r.Home}
	elem = append(elem, StaticDir, p)
	return r.relativePath(path.Join(elem...))
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
		if !isInternal(pkg.RelativePath) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered
}

func isInternal(relpath string) bool {
	return relpath == "internal" ||
		strings.HasPrefix(relpath, "internal/") ||
		strings.HasSuffix(relpath, "/internal") ||
		strings.Contains(relpath, "/internal/")
}

// dict turns key-value pairs into a map.
// Odd numbered arguments are keys, even numbered arguments are values.
func dict(args ...any) (map[string]any, error) {
	if len(args)%2 != 0 {
		return nil, errtrace.Wrap(fmt.Errorf("dict: odd number of arguments"))
	}
	dict := make(map[string]any, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			return nil, errtrace.Wrap(fmt.Errorf("dict: [%d] should be string, got %T", i, args[i]))
		}
		dict[key] = args[i+1]
	}
	return dict, nil
}
