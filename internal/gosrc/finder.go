package gosrc

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/multierr"
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
}

// Finder searches for and returns Go package references
// using the go/packages library.
//
// The zero value of this is ready to use.
type Finder struct {
	// Build tags to enable when searching for packages.
	Tags []string

	// Logger to write regular log messages to.
	Log *log.Logger

	// Logger to write debug messages to.
	//
	// Use nil to disable debug logging.
	DebugLog *log.Logger

	// Reference to packages.Load.
	//
	// May be overridden during tests.
	loadGoPackages func(*packages.Config, ...string) ([]*packages.Package, error)
}

const _finderLoadMode = packages.NeedName | packages.NeedCompiledGoFiles

// FindPackages searches for packages matching the given import path patterns,
// and returns references to them.
func (f *Finder) FindPackages(patterns ...string) ([]*PackageRef, error) {
	cfg := packages.Config{
		Mode: _finderLoadMode,
		// We want to find tests as well,
		// but Tests can not be set to true
		// in NeedName/NeedCompiledGoFiles mode.
	}
	if ts := f.Tags; len(ts) > 0 {
		cfg.BuildFlags = append(cfg.BuildFlags, "-tags", strings.Join(ts, ","))
	}
	if f.DebugLog != nil {
		cfg.Logf = f.DebugLog.Printf
	}

	loadGoPackages := packages.Load
	if f.loadGoPackages != nil {
		loadGoPackages = f.loadGoPackages
	}

	pkgs, err := loadGoPackages(&cfg, patterns...)
	if err != nil {
		return nil, err
	}

	if len(pkgs) == 0 {
		return nil, errors.New("no packages found")
	}

	infos := make([]*PackageRef, 0, len(pkgs))
	var resultErr error
	for _, pkg := range pkgs {
		if err := combinePackageErrors(pkg); err != nil {
			resultErr = multierr.Append(resultErr, err)
			continue
		}

		if len(pkg.CompiledGoFiles) == 0 {
			f.Log.Printf("[%v] Skipping: no files found.", pkg.PkgPath)
			continue
		}

		pkgDir := filepath.Dir(pkg.CompiledGoFiles[0])
		var testFiles []string
		if ents, err := os.ReadDir(pkgDir); err != nil {
			f.Log.Printf("[%v] Skipping tests: unable to read directory: %v", pkg.PkgPath, err)
		} else {
			for _, ent := range ents {
				if !ent.IsDir() && strings.HasSuffix(ent.Name(), "_test.go") {
					testFiles = append(testFiles, filepath.Join(pkgDir, ent.Name()))
				}
			}
		}

		infos = append(infos, &PackageRef{
			Name:       pkg.Name,
			ImportPath: pkg.PkgPath,
			Files:      pkg.CompiledGoFiles,
			TestFiles:  testFiles,
		})
	}
	return infos, resultErr
}

func combinePackageErrors(pkg *packages.Package) error {
	var errs error
	for _, perr := range pkg.Errors {
		errs = multierr.Append(errs, perr)
	}
	if errs != nil {
		return fmt.Errorf("package %v (%v): %w", pkg.Name, pkg.PkgPath, errs)
	}
	return nil
}
