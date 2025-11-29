package main

import (
	"bytes"
	"go/doc/comment"
	"strings"
	"text/template"

	"go.abhg.dev/doc2go/internal/pathtree"
	"go.abhg.dev/doc2go/internal/relative"
)

type docLinker struct {
	knownImports map[string]struct{}
	templates    pathtree.Root[*template.Template]

	RelLinkStyle relLinkStyle
	Basename     string
}

// LocalPackage marks an import path as a "local" package.
//
// A local package is part of the current documentation generation scope,
// so links to these packages will be relative.
func (rl *docLinker) LocalPackage(importPath string) {
	if rl.knownImports == nil {
		rl.knownImports = make(map[string]struct{})
	}
	rl.knownImports[importPath] = struct{}{}
}

// Template specifies a package documentation template
// for packages at this import path and its descendants.
func (rl *docLinker) Template(path string, tmpl *template.Template) {
	rl.templates.Set(path, tmpl)
}

func (rl *docLinker) packageDocURL(fromPkg, pkg string) string {
	if _, ok := rl.knownImports[pkg]; ok {
		return rl.RelLinkStyle.Normalize(relative.Path(fromPkg, pkg), rl.Basename)
	}

	if tmpl, ok := rl.templates.Lookup(pkg); ok {
		d := struct{ ImportPath string }{ImportPath: pkg}
		var buff bytes.Buffer
		if err := tmpl.Execute(&buff, d); err == nil {
			return strings.TrimSpace(buff.String())
		}
		// TODO: log the error
	}

	return "https://pkg.go.dev/" + pkg
}

func (rl *docLinker) DocLinkURL(fromPkg string, l *comment.DocLink) string {
	var sb strings.Builder
	if l.ImportPath != "" {
		sb.WriteString(rl.packageDocURL(fromPkg, l.ImportPath))
	}
	if len(l.Recv) > 0 {
		sb.WriteRune('#')
		sb.WriteString(l.Recv)
		sb.WriteRune('.')
	}
	if len(l.Name) > 0 {
		if len(l.Recv) == 0 {
			sb.WriteRune('#')
		}
		sb.WriteString(l.Name)
	}
	return sb.String()
}
