package gomod

import (
	"log"
	"os"

	"braces.dev/errtrace"
	"go.abhg.dev/doc2go/internal/gosrc"
	"golang.org/x/mod/modfile"
)

// Builder builds a [Tree] containing module data for the workspace
// based on the given packages.
type Builder struct {
	Logger *log.Logger // required
}

// Build creates a Tree populated with module dependencies
// from the go.mod files of the given packages.
//
// The returned Tree may be empty.
func (b *Builder) Build(pkgs []*gosrc.PackageRef) *Tree {
	// Multiple packages can be part of the same module,
	// so use a map to deduplicate module references.
	seen := make(map[string]struct{}) // set[module path]

	// TODO: better for FindPackages to return unique modules directly.
	var tree Tree
	for _, pkg := range pkgs {
		mod := pkg.Module
		if mod == nil {
			continue // no module information available
		}
		if _, ok := seen[mod.Path]; ok {
			continue
		}

		deps, err := parseGomod(mod)
		if err != nil {
			b.Logger.Printf("warning: error parsing go.mod %q: %v", mod.GoMod, err)
			continue
		}

		tree.PutModuleDeps(mod.Path, deps)
	}

	return &tree
}

func parseGomod(ref *gosrc.ModuleRef) (deps []*Module, err error) {
	data, err := os.ReadFile(ref.GoMod)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	mf, err := modfile.Parse(ref.GoMod, data, nil)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	if mf.Module == nil {
		return nil, errtrace.Errorf("no module directive in %s", ref.GoMod)
	}

	replacements := make(map[string]string) // module path -> version
	for _, repl := range mf.Replace {
		if repl.New.Version != "" {
			replacements[repl.Old.Path] = repl.New.Version
		}
	}
	for _, req := range mf.Require {
		version := req.Mod.Version
		if replVersion, ok := replacements[req.Mod.Path]; ok {
			version = replVersion
		}

		deps = append(deps, &Module{
			Path:    req.Mod.Path,
			Version: version,
		})
	}

	return deps, nil
}
