package main

import (
	"bytes"
	"go/doc/comment"
	"strings"
	"text/template"

	"go.abhg.dev/doc2go/internal/pathtree"
	"go.abhg.dev/doc2go/internal/relative"
)

type templateTree = pathtree.Root[*template.Template]

type docLinker struct {
	knownImports map[string]struct{}
	templates    templateTree
}

func (rl *docLinker) packageDocURL(fromPkg, pkg string) string {
	if _, ok := rl.knownImports[pkg]; ok {
		return relative.Path(fromPkg, pkg)
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
