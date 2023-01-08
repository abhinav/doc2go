package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.abhg.dev/doc2go/internal/flagvalue"
)

var (
	errHelp             = flag.ErrHelp
	errInvalidArguments = errors.New("invalid arguments")
)

// params holds all arguments for doc2go.
type params struct {
	version bool
	help    Help

	Tags  string
	Debug flagvalue.FileSwitch

	Basename  string
	OutputDir string

	Embed       bool
	Internal    bool
	PkgDocs     []pathTemplate
	Frontmatter string

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
		DefaultHelp.Write(cmd.Stderr)
	}

	var p params

	// Filesystem:
	flag.StringVar(&p.OutputDir, "out", "_site", "")
	flag.StringVar(&p.Basename, "basename", "", "")

	// HTML output:
	flag.BoolVar(&p.Internal, "internal", false, "")
	flag.BoolVar(&p.Embed, "embed", false, "")
	flag.StringVar(&p.Frontmatter, "frontmatter", "", "")
	flag.Var(flagvalue.ListOf(&p.PkgDocs), "pkg-doc", "")

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

	if p.help == DefaultHelp && len(args) > 0 {
		// The user might have done "-h foo"
		// instead of "-h=foo".
		// If the argument is a known help topic,
		// take it.
		var h Help
		if err := h.Set(args[0]); err == nil {
			p.help = h
		}
	}

	switch p.help {
	case NoHelp:
		// proceed as usual
	default:
		if err := p.help.Write(cmd.Stderr); err != nil {
			fmt.Fprintln(cmd.Stderr, err)
		}
		return nil, errHelp
	}

	p.Patterns = args
	if len(p.Patterns) == 0 {
		fmt.Fprintln(cmd.Stderr, "Please provide at least one pattern.")
		UsageHelp.Write(cmd.Stderr)
		return nil, errInvalidArguments
	}

	return p, nil
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
