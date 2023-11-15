package gosrc

import (
	"cmp"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.abhg.dev/doc2go/internal/sliceutil"
	"golang.org/x/tools/go/packages"
)

// PackageRef is a reference to a package.
//
// It holds information necessary to load a package,
// but doesn't yet load it.
type PackageRef struct {
	// Name of the package.
	Name string

	// Import path of the package.
	ImportPath string

	// List of .go files in the package.
	Files []string

	// List of _test.go files in the package.
	TestFiles []string

	// Packages imported by this package.
	Imports []ImportedPackage
}

// ImportedPackage is a package imported by another package.
type ImportedPackage struct {
	Name       string
	ImportPath string
}

// Finder searches for and returns Go package references
// using the go/packages library.
//
// The zero value of this is ready to use.
type Finder struct {
	PackagesConfig *packages.Config

	// Build tags to enable when searching for packages.
	Tags []string

	// Logger to write regular log messages to.
	Log *log.Logger

	// Logger to write debug messages to.
	//
	// Use nil to disable debug logging.
	DebugLog *log.Logger
}

const _finderLoadMode = packages.NeedName | packages.NeedFiles | packages.NeedImports

// FindPackages searches for packages matching the given import path patterns,
// and returns references to them.
func (f *Finder) FindPackages(patterns ...string) ([]*PackageRef, error) {
	var cfg packages.Config
	if f.PackagesConfig != nil {
		cfg = *f.PackagesConfig
	}

	// We want to find tests as well,
	// but Tests can not be set to true
	// in NeedName/NeedFiles mode.
	cfg.Mode = _finderLoadMode
	cfg.Tests = false
	if ts := f.Tags; len(ts) > 0 {
		cfg.BuildFlags = append(cfg.BuildFlags, "-tags", strings.Join(ts, ","))
	}
	if f.DebugLog != nil {
		cfg.Logf = f.DebugLog.Printf
	}

	pkgs, err := packages.Load(&cfg, patterns...)
	if err != nil {
		return nil, err
	}

	if len(pkgs) == 0 {
		return nil, errors.New("no packages found")
	}

	infos := make([]*PackageRef, 0, len(pkgs))
	for _, pkg := range pkgs {
		var pkgFailed bool
		for _, err := range pkg.Errors {
			pkgFailed = true
			f.Log.Printf("[%v] %v", pkg.PkgPath, err)
		}
		if pkgFailed {
			continue
		}

		goFiles := sliceutil.RemoveFunc(pkg.GoFiles,
			func(path string) bool {
				return !strings.HasSuffix(path, ".go")
			})

		if len(goFiles) == 0 {
			f.Log.Printf("[%v] No non-test Go files. Skipping.", pkg.PkgPath)
			continue
		}

		pkgDir := filepath.Dir(goFiles[0])
		var testFiles []string
		if ents, err := os.ReadDir(pkgDir); err != nil {
			f.Log.Printf("[%v] Skipping tests: unable to read directory: %v", pkg.PkgPath, err)
		} else {
			// FIXME: This ignores build tags in test files.
			// Maybe, it should be two load calls:
			// find and then,
			// for each package, list files and test files.
			for _, ent := range ents {
				if !ent.IsDir() && strings.HasSuffix(ent.Name(), "_test.go") {
					testFiles = append(testFiles, filepath.Join(pkgDir, ent.Name()))
				}
			}
		}

		var imports []ImportedPackage
		if len(pkg.Imports) > 0 {
			imports = make([]ImportedPackage, 0, len(pkg.Imports))
			for _, imp := range pkg.Imports {
				imports = append(imports, ImportedPackage{
					Name:       imp.Name,
					ImportPath: imp.PkgPath,
				})
			}
		}
		slices.SortFunc(imports, func(i, j ImportedPackage) int {
			return cmp.Compare(i.ImportPath, j.ImportPath)
		})

		infos = append(infos, &PackageRef{
			Name:       pkg.Name,
			ImportPath: pkg.PkgPath,
			Files:      goFiles,
			TestFiles:  testFiles,
			Imports:    imports,
		})
	}
	return infos, nil
}
