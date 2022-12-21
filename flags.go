package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"strings"
	"text/template"

	"go.abhg.dev/doc2go/internal/flagvalue"
)

const _usage = `usage: doc2go [options] pattern ...`

// params holds all arguments for doc2go.
type params struct {
	Tags      string
	Debug     bool
	OutputDir string
	Links     []pathTemplate
	Patterns  []string
	Internal  bool
}

// cliParser parses the command line arguments for doc2go.
type cliParser struct {
	Stderr io.Writer
	Log    *log.Logger
}

func (cmd *cliParser) printShortUsage() {
	cmd.Log.Println(_usage)
}

var errInvalidArguments = errors.New("invalid arguments")

func (cmd *cliParser) parseParams(args []string) (*params, error) {
	flag := flag.NewFlagSet("doc2go", flag.ContinueOnError)
	flag.SetOutput(cmd.Stderr)
	flag.Usage = func() {
		cmd.printShortUsage()
		cmd.Log.Println("The following options are available:")
		flag.PrintDefaults()
	}

	var p params
	flag.StringVar(&p.OutputDir, "out", "_site", "Write files to `dir`.")
	flag.Var(flagvalue.ListOf(&p.Links), "link",
		"Given `path=template`, use 'template' to link to documentation\n"+
			"for import paths under 'path'.\n"+
			"This flag may be provided multiple times.")
	flag.BoolVar(&p.Internal, "internal", false,
		"Include internal packages in package listings.")
	flag.BoolVar(&p.Debug, "debug", false, "Print debugging output")
	flag.StringVar(&p.Tags, "tags", "", "List of comma-separated build `tags`")

	if err := flag.Parse(args); err != nil {
		return nil, err
	}

	p.Patterns = flag.Args()
	if len(p.Patterns) == 0 {
		cmd.Log.Println("Please provide at least one pattern.")
		cmd.printShortUsage()
		return nil, errInvalidArguments
	}

	return &p, nil
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
		return fmt.Errorf("parse template: %w", err)
	}

	return nil
}
