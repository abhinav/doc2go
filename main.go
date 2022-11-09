package main

import (
	"errors"
	"flag"
	"go/doc/comment"
	"io"
	"log"
	"os"
	"strings"
	"text/template"

	"go.abhg.dev/doc2go/internal/godoc"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/html"
	"go.abhg.dev/doc2go/internal/pathtree"
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

	linkTmpl := newTemplateTree(_defaultLinkTemplate)
	for _, lt := range opts.Links {
		linkTmpl.Set(lt.Path, lt.Template)
	}

	runner := Runner{
		Log:       cmd.log,
		Finder:    &finder,
		Parser:    new(gosrc.Parser),
		Assembler: new(godoc.Assembler),
		Renderer: &html.Renderer{
			DocPrinter: &comment.Printer{},
		},
		OutDir: opts.OutputDir,
	}

	return runner.Run(opts.Patterns)
}

type templateTree struct {
	tree     pathtree.Root[*template.Template]
	fallback *template.Template
}

func newTemplateTree(fallback *template.Template) *templateTree {
	return &templateTree{fallback: fallback}
}

func (t *templateTree) Set(p string, v *template.Template) {
	t.tree.Set(p, v)
}

func (t *templateTree) Get(p string) *template.Template {
	v, ok := t.tree.Lookup(p)
	if ok {
		v = t.fallback
	}
	return v
}

// TODO: our own template data
var _defaultLinkTemplate = template.Must(template.New("default").Parse(
	`
{{- with .ImportPath -}}
	https://pkg.go.dev/{{ . }}
{{- end -}}
{{- if or .Recv .Name -}}
	#
	{{- if .Recv -}}
		{{ .Recv }}{{ with .Name }}.{{ . }}{{ end }}
	{{- else -}}
		{{ .Name }}
	{{- end -}}
{{- end -}}
`))

// TODO: Use a local doc linker for things within scope of the render.
// So this and comment.Printer have to be constructed per-package.
// func docLinker(t *templateTree) func(*comment.DocLink) string {
// 	return func(dl *comment.DocLink) string {

// 	}
// }
