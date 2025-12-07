package main

import (
	"bytes"
	"go/doc/comment"
	"strings"
	"text/template"

	"go.abhg.dev/doc2go/internal/gomod"
	"go.abhg.dev/doc2go/internal/pathtree"
	"go.abhg.dev/doc2go/internal/relative"
)

// ModuleLookuper provides module version information for import paths.
type ModuleLookuper interface {
	// LookupModuleDep returns module information for the given import path
	// based on the source package's module context.
	//
	// sourceImportPath determines which go.mod file to use.
	// targetImportPath is the dependency being linked to.
	//
	// Returns nil if the target is not a known dependency.
	LookupModuleDep(sourceImportPath, targetImportPath string) *gomod.Module
}

var _ ModuleLookuper = (*gomod.Tree)(nil)

type docLinker struct {
	knownImports map[string]struct{}
	templates    pathtree.Root[*template.Template]

	RelLinkStyle relLinkStyle
	Basename     string

	// ModuleTree provides module version information for external dependencies.
	// If nil, external links are generated without version information.
	ModuleTree ModuleLookuper
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

type packageDocTemplateData struct {
	// ImportPath of the package being linked to.
	ImportPath string // required

	// Module is the module information for the package.
	// If the package is not part of a known module,
	// this will be nil.
	Module *packageDocTemplateModuleData
}

type packageDocTemplateModuleData struct {
	// Path is the module path.
	Path string // required

	// Version is the module version.
	Version string // required

	// Subpath is the import path relative to the module root.
	// Empty if ImportPath equals the module path.
	Subpath string
}

func (rl *docLinker) packageDocURL(fromPkg, pkg string) string {
	if _, ok := rl.knownImports[pkg]; ok {
		return rl.RelLinkStyle.Normalize(relative.Path(fromPkg, pkg), rl.Basename)
	}

	var (
		modInfo    *gomod.Module // module pkg belongs to, if any
		modSubpath string        // subpath within module to get back to pkg
	)
	if rl.ModuleTree != nil {
		modInfo = rl.ModuleTree.LookupModuleDep(fromPkg, pkg)
		if modInfo != nil && pkg != modInfo.Path {
			modSubpath = strings.TrimPrefix(pkg, modInfo.Path+"/")
		}
	}

	if tmpl, ok := rl.templates.Lookup(pkg); ok {
		d := packageDocTemplateData{
			ImportPath: pkg,
		}

		// Populate module information if available.
		if modInfo != nil {
			d.Module = &packageDocTemplateModuleData{
				Path:    modInfo.Path,
				Version: modInfo.Version,
				Subpath: modSubpath,
			}
		}

		var buff bytes.Buffer
		if err := tmpl.Execute(&buff, d); err == nil {
			return strings.TrimSpace(buff.String())
		}
		// TODO: log the error
	}

	// Fall back to pkg.go.dev.
	//
	// If we have module information,
	// generate a versioned link for pkg.go.dev.
	var link strings.Builder
	link.WriteString("https://pkg.go.dev/")
	if modInfo != nil {
		link.WriteString(modInfo.Path)
		link.WriteByte('@')
		link.WriteString(modInfo.Version)
		if modSubpath != "" {
			link.WriteByte('/')
			link.WriteString(modSubpath)
		}
	} else {
		// No module info, just use unversioned link.
		link.WriteString(pkg)
	}
	return link.String()
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
