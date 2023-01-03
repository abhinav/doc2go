package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/template"

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

	PackageDocTemplates []pathTemplate
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

func (cmd *cliParser) newFlagSet() (*params, *flag.FlagSet) {
	flag := flag.NewFlagSet("doc2go", flag.ContinueOnError)
	flag.SetOutput(cmd.Stderr)
	flag.Usage = func() {
		fmt.Fprintln(cmd.Stderr, _shortUsage)
		fmt.Fprint(cmd.Stderr, _about+"\n")
		fmt.Fprint(cmd.Stderr, "OPTIONS\n\n")
		flag.PrintDefaults()
	}

	var p params
	flag.StringVar(&p.OutputDir, "out", "_site", "write files to `DIR`.")
	flag.BoolVar(&p.Internal, "internal", false, "include internal packages in package listings.\n"+
		"We always generate documentation for internal packages,\n"+
		"but by default, we do not include them in package lists.\n"+
		"Use this flag to have them listed.")
	flag.BoolVar(&p.Embedded, "embed", false, "generate partial HTML pages fit for embedding.\n"+
		"Instead of generating a standalone HTML website, generate partial HTML pages\n"+
		"that can be incorporated into a website using a static site generator.")
	flag.Var(flagvalue.ListOf(&p.PackageDocTemplates), "pkg-doc", "use TEMPLATE to generate documentation links for PATH and its children.\n"+
		"  -pkg-doc example.com=https://godoc.example.com/{{.ImportPath}}\n"+
		"The argument must be in the form `PATH=TEMPLATE`.\n"+
		"The template is a text/template that gets the following variables:\n"+
		"  ImportPath: import path of the package\n"+
		"Pass this in multiple times to specify different patterns\n"+
		"for different import path scopes.")
	flag.Var(&p.Debug, "debug", "print debugging output to stderr or FILE,\n"+
		"if specified in the form -debug=FILE.")
	flag.StringVar(&p.Tags, "tags", "", "list of comma-separated build tags.")
	return &p, flag
}

func (cmd *cliParser) Parse(args []string) (*params, error) {
	p, flag := cmd.newFlagSet()
	version := flag.Bool("version", false, "report the tool version.")
	if err := flag.Parse(args); err != nil {
		return nil, err
	}

	if *version {
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
	Template *template.Template
	rawTmpl  string
}

var _ flag.Getter = (*pathTemplate)(nil)

func (pt *pathTemplate) Get() any { return pt }

func (pt *pathTemplate) String() string {
	return fmt.Sprintf("%s=%s", pt.Path, pt.rawTmpl)
}

func (pt *pathTemplate) Set(s string) error {
	idx := strings.IndexRune(s, '=')
	if idx < 0 {
		return fmt.Errorf("expected form 'path=template'")
	}

	pt.Path = s[:idx]
	pt.rawTmpl = s[idx+1:]

	var err error
	pt.Template, err = template.New(pt.Path).Parse(pt.rawTmpl)
	if err != nil {
		return fmt.Errorf("bad template: %w", err)
	}

	return nil
}
