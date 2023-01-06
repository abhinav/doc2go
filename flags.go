package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.abhg.dev/doc2go/internal/flagvalue"
)

var errHelp = flag.ErrHelp

const _shortUsage = `USAGE: doc2go [OPTIONS] PATTERN ...`

// params holds all arguments for doc2go.
type params struct {
	Tags      string
	Debug     flagvalue.FileSwitch
	OutputDir string
	Patterns  []string
	Internal  bool
	Embedded  bool
	Basename  string

	PackageDocTemplates []pathTemplate

	version bool
}

// cliParser parses the command line arguments for doc2go.
type cliParser struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (cmd *cliParser) printShortUsage() {
	fmt.Fprintln(cmd.Stderr, _shortUsage)
}

var errInvalidArguments = errors.New("invalid arguments")

const _about = `
Generates API documentation for packages matching PATTERNs.
Specify ./... to match the package in the current directory
and all its descendants.

	doc2go ./...
`

//go:embed flags.txt
var _flagDefaults string

func (cmd *cliParser) newFlagSet() (*params, *flag.FlagSet) {
	flag := flag.NewFlagSet("doc2go", flag.ContinueOnError)
	flag.SetOutput(cmd.Stderr)
	flag.Usage = func() {
		fmt.Fprintln(cmd.Stderr, _shortUsage)
		fmt.Fprint(cmd.Stderr, _about+"\n")
		fmt.Fprint(cmd.Stderr, "OPTIONS\n\n")
		fmt.Fprint(cmd.Stderr, _flagDefaults)
	}

	var p params

	// Filesystem:
	flag.StringVar(&p.OutputDir, "out", "_site", "")
	flag.StringVar(&p.OutputDir, "o", "_site", "")
	flag.StringVar(&p.Basename, "basename", "", "")

	// HTML output:
	flag.BoolVar(&p.Internal, "internal", false, "")
	flag.BoolVar(&p.Embedded, "embed", false, "")
	flag.Var(flagvalue.ListOf(&p.PackageDocTemplates), "pkg-doc", "")

	// Go build system:
	flag.StringVar(&p.Tags, "tags", "", "")

	// Program-level:
	flag.Var(&p.Debug, "debug", "")
	flag.BoolVar(&p.version, "version", false, "")

	return &p, flag
}

func (cmd *cliParser) Parse(args []string) (*params, error) {
	p, flag := cmd.newFlagSet()
	if err := flag.Parse(args); err != nil {
		return nil, err
	}

	if p.version {
		fmt.Fprintln(cmd.Stdout, "doc2go", _version)
		return nil, errHelp
	}

	p.Patterns = flag.Args()
	if len(p.Patterns) == 0 {
		fmt.Fprintln(cmd.Stderr, "Please provide at least one pattern.")
		cmd.printShortUsage()
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
