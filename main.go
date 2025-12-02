// doc2go generates static HTML documentation from one or more Go packages.
//
// See https://abhinav.github.io/doc2go/ for information on how to use it.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"braces.dev/errtrace"
	"github.com/alecthomas/chroma/v2/styles"
	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/gomod"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/highlight"
	"go.abhg.dev/doc2go/internal/html"
	"go.abhg.dev/doc2go/internal/pagefind"
	"go.abhg.dev/doc2go/internal/pathx"
	"golang.org/x/tools/go/packages"
)

var _version = "dev"

func init() {
	// Zero out the Chroma style fallback
	// so that we know which theme we're using explicitly.
	styles.Fallback = nil
}

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
		fmt.Fprintf(cmd.Stderr, "%+v\n", err)
		return 1
	}

	debugw, closedebug, err := opts.Debug.Create(cmd.Stderr)
	if err != nil {
		cmd.log.Printf("Unable to create debug log, using stderr: %+v", err)
		debugw = cmd.Stderr
	} else {
		defer func() {
			if err := closedebug(); err != nil {
				cmd.log.Printf("Error closing debug log: %+v", err)
			}
		}()
	}
	cmd.debug = opts.Debug.Bool()
	cmd.debugLog = log.New(debugw, "", 0)

	// Change directory if -C flag is specified.
	if opts.Dir != "" {
		if err := os.Chdir(opts.Dir); err != nil {
			cmd.log.Printf("doc2go: %+v", err)
			return 1
		}
	}

	// TODO:
	// In the future, tie this context to a signal handler.
	ctx := context.Background()
	if err := cmd.run(ctx, opts); err != nil {
		cmd.log.Printf("doc2go: %+v", err)
		return 1
	}
	return 0
}

func (cmd *mainCmd) run(ctx context.Context, opts *params) error {
	if opts.HighlightListThemes {
		for _, name := range styles.Names() {
			fmt.Fprintln(cmd.Stdout, name)
		}
		return nil
	}

	highlighter := highlight.Highlighter{
		Style: _defaultStyle,
		// By default, we use classes in standalone mode
		// and inline styles in embedded mode.
		// Users can override this with
		// --highlight=classes:$theme or --highlight=inline:$theme
		UseClasses: !opts.Embed,
	}
	if theme := opts.Highlight.Theme; len(theme) > 0 {
		style := styles.Get(theme)
		if style == nil {
			if opts.HighlightPrintCSS {
				return errtrace.Wrap(fmt.Errorf("unknown theme %q", theme))
			}

			cmd.log.Printf("Unknown theme %q. Falling back to %q.", theme, _defaultStyle.Name)
			style = _defaultStyle
		}
		highlighter.Style = style
	}
	if mode := opts.Highlight.Mode; mode != highlightModeAuto {
		highlighter.UseClasses = mode == highlightModeClasses
	}

	if opts.HighlightPrintCSS {
		return errtrace.Wrap(highlighter.WriteCSS(cmd.Stdout))
	}

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
		return errtrace.Wrap(fmt.Errorf("find packages: %w", err))
	}

	if home := opts.Home; home != "" {
		refs := pkgRefs[:0]
		for _, r := range pkgRefs {
			if !pathx.Descends(home, r.ImportPath) {
				cmd.log.Printf("[%s] Not rooted under %v. Skipping.", r.ImportPath, home)
				continue
			}
			refs = append(refs, r)
		}
		pkgRefs = refs
	}

	// Build module dependency tree for versioned external links.
	linker := docLinker{
		RelLinkStyle: opts.RelLinkStyle,
		Basename:     opts.Basename,
	}
	if !opts.NoModuleVersions && len(pkgRefs) > 0 {
		builder := &gomod.Builder{Logger: cmd.log}
		if moduleTree := builder.Build(pkgRefs); moduleTree != nil {
			linker.ModuleTree = moduleTree
		}
	}
	for _, lt := range opts.PkgDocs {
		t, err := template.New(lt.Path).Parse(lt.Template)
		if err != nil {
			return errtrace.Wrap(fmt.Errorf("bad package documentation template %q: %w", lt.String(), err))
		}
		linker.Template(lt.Path, t)
	}
	for _, ref := range pkgRefs {
		linker.LocalPackage(ref.ImportPath)
	}

	var frontmatter *template.Template
	if path := opts.FrontMatter; len(path) > 0 {
		bs, err := os.ReadFile(path)
		if err != nil {
			return errtrace.Wrap(fmt.Errorf("-frontmatter: %w", err))
		}

		frontmatter, err = template.New(path).Parse(string(bs))
		if err != nil {
			return errtrace.Wrap(fmt.Errorf("bad frontmatter template: %w\n%s", err, bs))
		}
	}

	var indexer PageIndexer
	if p := opts.Pagefind; p.Mode != pagefindDisabled {
		enable := p.Mode == pagefindEnabled

		// If no explicitly enabled, and not in embed mode,
		// enable only if the pagefind binary is available.
		pagefindPath := p.Path
		if !enable && p.Mode == pagefindAuto && !opts.Embed {
			pagefindPath, err = exec.LookPath("pagefind")
			if err == nil {
				enable = true
			}
		}

		if enable {
			indexer = &pagefind.CLI{
				Pagefind: pagefindPath,
				Log:      cmd.debugLog,
			}
		}
	}

	parser := gosrc.Parser{
		Logger: cmd.log,
	}

	g := Generator{
		Home:     opts.Home,
		DebugLog: cmd.debugLog,
		Parser:   &parser,
		Assembler: &godoc.Assembler{
			Linker: &linker,
			Lexer:  highlight.GoLexer,
			Logger: cmd.log,
		},
		Pagefind: indexer,
		Renderer: &html.Renderer{
			Home:        opts.Home,
			Embedded:    opts.Embed,
			Internal:    opts.Internal,
			FrontMatter: frontmatter,
			Highlighter: &highlighter,
			NormalizeRelativePath: func(s string) string {
				return opts.RelLinkStyle.Normalize(s, opts.Basename)
			},
			Pagefind: indexer != nil,
		},
		OutDir:     opts.OutputDir,
		SubDir:     opts.SubDir,
		PkgVersion: opts.PkgVersion,
		Basename:   opts.Basename,
		DocLinker:  &linker,
	}

	return errtrace.Wrap(g.Generate(ctx, pkgRefs))
}
