package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2/styles"
	"go.abhg.dev/doc2go/internal/flagvalue"
)

var (
	errHelp             = flag.ErrHelp
	errInvalidArguments = errors.New("invalid arguments")

	_defaultStyle = styles.GitHub
)

// params holds all arguments for doc2go.
type params struct {
	version bool
	help    Help

	Tags  string
	Debug flagvalue.FileSwitch

	Basename  string
	OutputDir string
	Home      string

	Embed       bool
	Internal    bool
	PkgDocs     []pathTemplate
	FrontMatter string

	Highlight           highlightParams
	HighlightPrintCSS   bool
	HighlightListThemes bool

	// Empty only in alternative execution modes:
	//	-highlight-print-css
	//	-highlight-list-themes
	Patterns []string
}

// cliParser parses the command line arguments for doc2go.
type cliParser struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (cmd *cliParser) newFlagSet() (*params, *flag.FlagSet) {
	flag := flag.NewFlagSet("doc2go", flag.ContinueOnError)
	flag.SetOutput(cmd.Stderr)
	flag.Usage = func() {
		Help("default").Write(cmd.Stderr)
	}

	var p params

	// Filesystem:
	flag.StringVar(&p.OutputDir, "out", "_site", "")
	flag.StringVar(&p.Basename, "basename", "", "")
	flag.StringVar(&p.Home, "home", "", "")

	// HTML output:
	flag.BoolVar(&p.Internal, "internal", false, "")
	flag.BoolVar(&p.Embed, "embed", false, "")
	flag.StringVar(&p.FrontMatter, "frontmatter", "", "")
	flag.Var(flagvalue.ListOf(&p.PkgDocs), "pkg-doc", "")

	// Highlighting:
	flag.Var(&p.Highlight, "highlight", "")
	flag.BoolVar(&p.HighlightPrintCSS, "highlight-print-css", false, "")
	flag.BoolVar(&p.HighlightListThemes, "highlight-list-themes", false, "")

	// Go build system:
	flag.StringVar(&p.Tags, "tags", "", "")

	// Program-level:
	flag.Var(&p.Debug, "debug", "")
	flag.BoolVar(&p.version, "version", false, "")
	flag.Var(&p.help, "help", "")
	flag.Var(&p.help, "h", "")

	return &p, flag
}

func (cmd *cliParser) Parse(args []string) (*params, error) {
	p, flag := cmd.newFlagSet()
	if err := flag.Parse(args); err != nil {
		return nil, err
	}
	args = flag.Args()

	if p.version {
		fmt.Fprintln(cmd.Stdout, "doc2go", _version)
		return nil, errHelp
	}

	if p.help == "default" && len(args) > 0 {
		// The user might have done "-h foo"
		// instead of "-h=foo".
		// If the argument is a known help topic,
		// take it.
		var h Help
		if err := h.Set(args[0]); err == nil {
			p.help = h
		}
	}

	if len(p.help) != 0 {
		if err := p.help.Write(cmd.Stderr); err != nil {
			fmt.Fprintln(cmd.Stderr, err)
		}
		return nil, errHelp
	}

	p.Patterns = args
	if len(p.Patterns) == 0 && !p.HighlightPrintCSS && !p.HighlightListThemes {
		fmt.Fprintln(cmd.Stderr, "Please provide at least one pattern.")
		Help("usage").Write(cmd.Stderr)
		return nil, errInvalidArguments
	}

	return p, nil
}

type highlightMode int

const (
	highlightModeAuto highlightMode = iota
	highlightModeClasses
	highlightModeInline
)

func (m highlightMode) String() string {
	switch m {
	case highlightModeAuto:
		return "auto"
	case highlightModeClasses:
		return "classes"
	case highlightModeInline:
		return "inline"
	default:
		return fmt.Sprintf("highlightMode(%d)", int(m))
	}
}

func (m *highlightMode) Set(s string) error {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "auto":
		*m = highlightModeAuto
	case "classes":
		*m = highlightModeClasses
	case "inline":
		*m = highlightModeInline
	default:
		return fmt.Errorf("unrecognized highlight mode %q", s)
	}
	return nil
}

type highlightParams struct {
	Mode  highlightMode
	Theme string
}

var _ flag.Getter = (*highlightParams)(nil)

func (p *highlightParams) Get() any { return p }

func (p *highlightParams) String() string {
	return fmt.Sprintf("%v:%v", p.Mode, p.Theme)
}

func (p *highlightParams) Set(s string) error {
	if idx := strings.IndexRune(s, ':'); idx > 0 {
		if err := p.Mode.Set(s[:idx]); err != nil {
			return err
		}
		s = s[idx+1:]
	}
	p.Theme = s
	return nil
}

type pathTemplate struct {
	Path     string
	Template string
}

var _ flag.Getter = (*pathTemplate)(nil)

func (pt *pathTemplate) Get() any { return pt }

func (pt *pathTemplate) String() string {
	return fmt.Sprintf("%s=%s", pt.Path, pt.Template)
}

func (pt *pathTemplate) Set(s string) error {
	idx := strings.IndexRune(s, '=')
	if idx < 0 {
		return fmt.Errorf("expected form 'path=template'")
	}

	pt.Path = s[:idx]
	pt.Template = s[idx+1:]
	return nil
}
