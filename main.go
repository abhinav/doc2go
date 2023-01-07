// doc2go generates static HTML documentation from one or more Go packages.
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"text/template"

	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/html"
	"golang.org/x/tools/go/packages"
)

var _version = "dev"

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

	log      *log.Logger
	debugLog *log.Logger
	debug    bool

	packagesConfig *packages.Config
}

func (cmd *mainCmd) Run(args []string) (exitCode int) {
	cmd.log = log.New(cmd.Stderr, "", 0)

	opts, err := (&cliParser{
		Stdout: cmd.Stdout,
		Stderr: cmd.Stderr,
	}).Parse(args)
	if err != nil {
		// '$cmd -h' should exit with zero.
		if errors.Is(err, errHelp) {
			return 0
		}
		// No need to print anything.
		// cliParser.Parse prints messages.
		return 1
	}

	debugw, closedebug, err := opts.Debug.Create(cmd.Stderr)
	if err != nil {
		cmd.log.Printf("Unable to create debug log, using stderr: %v", err)
		debugw = cmd.Stderr
	} else {
		defer closedebug()
	}
	cmd.debug = opts.Debug.Bool()
	cmd.debugLog = log.New(debugw, "", 0)

	if err := cmd.run(opts); err != nil {
		cmd.log.Printf("doc2go: %v", err)
		return 1
	}
	return 0
}

func (cmd *mainCmd) run(opts *params) error {
	finder := gosrc.Finder{
		Tags:           strings.Split(opts.Tags, ","),
		Log:            cmd.log,
		PackagesConfig: cmd.packagesConfig,
	}
	if cmd.debug {
		finder.DebugLog = cmd.debugLog
	}

	pkgRefs, err := finder.FindPackages(opts.Patterns...)
	if err != nil {
		return fmt.Errorf("find packages: %w", err)
	}

	var linker docLinker
	for _, lt := range opts.PkgDocs {
		t, err := template.New(lt.Path).Parse(lt.Template)
		if err != nil {
			return fmt.Errorf("bad package documentation template %q: %w", lt.String(), err)
		}
		linker.Template(lt.Path, t)
	}
	for _, ref := range pkgRefs {
		linker.LocalPackage(ref.ImportPath)
	}

	var frontmatter *template.Template
	if path := opts.Frontmatter; len(path) > 0 {
		bs, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("-frontmatter: %w", err)
		}

		frontmatter, err = template.New(path).Parse(string(bs))
		if err != nil {
			return fmt.Errorf("bad frontmatter template: %w\n%s", err, bs)
		}
	}

	g := Generator{
		DebugLog: cmd.debugLog,
		Parser:   new(gosrc.Parser),
		Assembler: &godoc.Assembler{
			Linker: &linker,
		},
		Renderer: &html.Renderer{
			Embedded:    opts.Embed,
			Internal:    opts.Internal,
			Frontmatter: frontmatter,
		},
		OutDir:    opts.OutputDir,
		Basename:  opts.Basename,
		DocLinker: &linker,
	}

	return g.Generate(pkgRefs)
}
