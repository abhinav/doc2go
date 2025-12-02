package gomod

import (
	"go.abhg.dev/doc2go/internal/pathtree"
)

// Module identifies a Go module.
type Module struct {
	// Path is the module path (e.g., "go.uber.org/zap").
	Path string // required

	// Version is the module version (e.g., "v1.27.1").
	// This may be a pseudo-version.
	Version string // required
}

// Tree provides module version information inside a multi-module environment.
//
// As a workspace may have multiple go.mod files (e.g., in a monorepo),
// we need to resolve which go.mod file applies
// when looking at an import path.
// We do so by also looking at the source import path
// (i.e., the package importing the target),
// and finding _its_ go.mod file.
//
// So lookups are two-level:
//
//   - The first level resolves which go.mod file to use.
//   - The second level resolves the module path for an import path
//     within the selected go.mod file.
//
// Zero value of tree is an empty tree.
type Tree struct {
	// sources maps source import paths to their module's dependencies.
	//
	// Key: source module path (e.g., "example.com/org/foo")
	// at which the module's go.mod file is located.
	// Value: moduleVersions containing that module's go.mod dependencies
	// (e.g., "go.uber.org/zap", "github.com/stretchr/testify").
	//
	// This takes advantage of pathtree's ability to resolve prefixes
	// to find the deepest matching value.
	sources pathtree.Root[*moduleVersions]
}

// moduleVersions holds the dependency tree for a single go.mod file.
type moduleVersions struct {
	// deps maps dependency module paths to their version numbers.
	deps pathtree.Root[*Module]
}

// PutModuleDeps registers a module and its dependencies.
// modulePath is the module root (e.g., "example.com/myproject").
// deps is the list of dependencies from that module's go.mod.
func (t *Tree) PutModuleDeps(modulePath string, deps []*Module) {
	if modulePath == "" {
		return
	}

	mv, ok := t.sources.Lookup(modulePath)
	if !ok || mv == nil {
		mv = new(moduleVersions)
		t.sources.Set(modulePath, mv)
	}

	for _, dep := range deps {
		if dep != nil && dep.Path != "" {
			mv.deps.Set(dep.Path, dep)
		}
	}
}

// LookupModuleDep retreives information about the Go module
// that targetImportPath belongs to, as a dependency of sourceImportPath.
//
// This operates by finding the go.mod file for sourceImportPath,
// and then looking for a Go module inside its dependencies
// that matches targetImportPath.
//
// Returns nil if a known module dependency is not found.
func (t *Tree) LookupModuleDep(sourceImportPath, targetImportPath string) *Module {
	// Find the moduleVersions for the source package's module.
	// pathtree handles prefix matching, so "example.com/myproject/foo/bar"
	// will find the moduleVersions set at "example.com/myproject".
	mv, ok := t.sources.Lookup(sourceImportPath)
	if !ok || mv == nil {
		return nil
	}

	// Look up the target in this module's dependency tree.
	// Again, pathtree handles prefix matching, so
	// "go.uber.org/zap/zaptest" will find "go.uber.org/zap".
	info, ok := mv.deps.Lookup(targetImportPath)
	if !ok {
		return nil
	}

	return info
}
