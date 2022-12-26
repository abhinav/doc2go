// doc2go generates static HTML documentation from one or more Go packages.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/html"
)

func main() {
	cmd := mainCmd{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	os.Exit(cmd.Run(os.Args[1:]))
}

// mainCmd is the actual entry point to the program.
type mainCmd struct {
	Stdout io.Writer // == os.Stdout
	Stderr io.Writer // == os.Stderr

	log *log.Logger
}

func (cmd *mainCmd) Run(args []string) (exitCode int) {
	cmd.log = log.New(cmd.Stderr, "", 0)

	opts, err := (&cliParser{
		Stderr: cmd.Stderr,
		Log:    cmd.log,
	}).parseParams(args)
	if err != nil {
		// '$cmd -h' should exit with zero.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		// No need to print anything.
		// parseParams prints messages.
		return 1
	}

	if err := cmd.run(opts); err != nil {
		cmd.log.Printf("doc2go: %v", err)
		return 1
	}
	return 0
}

func (cmd *mainCmd) run(opts *params) error {
	finder := gosrc.Finder{
		Tags: strings.Split(opts.Tags, ","),
	}
	if opts.Debug {
		finder.DebugLog = cmd.log
	}

	pkgRefs, err := finder.FindPackages(opts.Patterns...)
	if err != nil {
		return fmt.Errorf("find packages: %w", err)
	}

	var pkgDocTmpls templateTree
	for _, lt := range opts.PackageDocTemplates {
		pkgDocTmpls.Set(lt.Path, lt.Template)
	}

	knownImports := make(map[string]struct{}, len(pkgRefs))
	for _, ref := range pkgRefs {
		knownImports[ref.ImportPath] = struct{}{}
	}

	linker := docLinker{
		knownImports: knownImports,
		templates:    pkgDocTmpls,
	}

	g := Generator{
		Log:    cmd.log,
		Finder: &finder,
		Parser: new(gosrc.Parser),
		Assembler: &godoc.Assembler{
			Linker: &linker,
		},
		Renderer:  new(html.Renderer),
		OutDir:    opts.OutputDir,
		Internal:  opts.Internal,
		DocLinker: &linker,
	}

	return g.Generate(pkgRefs)
}
