package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Help is doc2go's -h/-help flag.
// It supports retrieving help on various topics by passing in a parameter.
type Help string

var _helpTopics = make(map[Help]string)

const (
	// NoHelp indicates that no help was requested.
	NoHelp Help = ""

	// UsageHelp prints a single line usage of the command.
	UsageHelp Help = "usage"

	// DefaultHelp refers to the help message reported
	// when no topic is selected.
	DefaultHelp Help = "default"

	// FrontMatterHelp explains how to use the -frontmatter flag.
	FrontMatterHelp Help = "frontmatter"

	// PackageDocHelp explains how to use the -pkg-doc flag.
	PackageDocHelp Help = "pkg-doc"

	// HighlightHelp explains how to use the -highlight flag.
	HighlightHelp Help = "highlight"
)

const _usageHelp = `USAGE: doc2go [OPTIONS] PATTERN ...` + "\n"

func init() { _helpTopics[UsageHelp] = _usageHelp }

const _defaultHelp = _usageHelp + `
Generates API documentation for packages matching PATTERNs.
Specify ./... to match the package in the current directory
and all its descendants.

	doc2go ./...

OPTIONS

  -basename NAME
	base name of generated files. Defaults to index.html.
  -out DIR
	write files to DIR. Defaults to _site.
  -home PATH
	import path for the home page of the documentation.
	Packages that aren't descendants of this path will be omitted.
  -embed
	generate partial HTML pages fit for embedding.
  -internal
	include internal packages in package listings.
  -frontmatter FILE
	generate front matter in HTML files via template in FILE.
	See -help=frontmatter for more information.
  -highlight [MODE:][THEME]
	use THEME to highlight code blocks.
	MODE, if any, is one of 'auto', 'inline', and 'classes'
	and specifies the method of highlighting.
	See -help=highlight for more information.
  -highlight-list-themes
	print a list of available themes.
  -highlight-print-css
	print the CSS for the THEME specified in -highlight.
  -pkg-doc PATH=TEMPLATE
	generate links for PATH and its children via TEMPLATE.
	See -help=pkg-doc for more information.
  -tags TAG,...
	list of comma-separated build tags.
  -debug[=FILE]
	print debugging output to stderr or FILE, if specified.
  -version
	report the tool version.
  -h, -help
	prints this message.
`

func init() { _helpTopics[DefaultHelp] = _defaultHelp }

const _frontmatterHelp = `-frontmatter FILE

FILE specifies a text/template to generate front matter.
doc2go will execute the template for each generated page,
and put the result at the top of each file,
separated from the rest of the content by an empty line.

This flag is typically used with -embed
to make doc2go's output compatible with static site generators.

	-frontmatter=frontmatter.tmpl -embed

The template is executed with the following context:

	struct {
		// Path to the package or directory relative
		// to the module root.
		// This is empty for the root index page.
		Path string
		// Last component of Path.
		// This is empty for the root index page.
		Basename string
		// Number of packages or directories directly under Path.
		NumChildren int

		// The following fields are set only for packages.
		Package struct {
			// Name of the package. Empty for directories.
			Name string
			// First sentence of the package documentation,
			// if any.
			Synopsis string
		}
	}

For example:

	---
	# Give example.com/foo/bar, use 'bar' as the page title.
	# For the root page, use the title "API Reference."
	title: "{{ with .Basename }}{{ . }}{{ else }}API Reference{{ end }}"
	# If this package has documentation,
	# use its first sentence as the page description.
	{{ with .Package.Synopsis -}}
	  description: {{ printf "%q" . }}
	{{ end -}}
	---
`

func init() { _helpTopics[FrontMatterHelp] = _frontmatterHelp }

const _packageDocHelp = `-pkg-doc PATH=TEMPLATE

Use the -pkg-doc flag to teach doc2go where to find documentation
for dependencies.
PATH is an import path, and TEMPLATE is a text/template.
Packages with the import path PATH, and any package under it
will use TEMPLATE to generate links to themselves.

TEMPLATE is executed with the following context:

	struct {
		// Import path of the target package.
		ImportPath string
	}

For example:

	-pkg-doc example.com=https://go.example.com/{{.ImportPath}}

This will use go.example.com for all packages under example.com.

	example.com/foo     => https://go.example.com/example.com/foo
	example.com/bar/baz => https://go.example.com/example.com/bar/baz

Pass this flag multiple times to set different templates
for different package scopes.
If two PATHs overlap, the more specific of the two will be used.
For example, given:

	-pkg-doc golang.org/x=https://godocs.io/{{.ImportPath}}
	-pkg-doc golang.org/x/tools=https://pkg.go.dev/{{.ImportPath}}

All packages under golang.org/x/ will use https://godocs.io,
except golang.org/x/tools which will use https://pkg.go.dev.
`

func init() { _helpTopics[PackageDocHelp] = _packageDocHelp }

var _highlightHelp = `-highlight [MODE:][THEME]

MODE determines the method of highlighting used.
Valid values of MODE are:

  auto
	Decide automatically based on other flags.
	Uses 'inline' if -embed is set, and 'classes' otherwise.
  classes
	Highlighting is performed via CSS classes.
  inline
	Highlighting is performed via inline 'style' tags on elements.

MODE may be omitted, in which case it defaults to 'auto'.

THEME specifies the theme used for highlighting.
THEME may be omitted, in which case it defaults to '` + _defaultStyle.Name + `'.
Get a list of available themes with the following command:

	doc2go -highlight-list-themes

You can preview most themes at https://swapoff.org/chroma/playground/.
doc2go additionally includes a minimal 'plain' theme
if you prefer not to have significant syntax highlighting.

In 'classes' mode, the theme's stylesheet must be included in the page for
highlighting to work. This is done automatically if -embed is not set.
If -embed is set, this must be done manually.
Use the -highlight-print-css flag to access a theme's stylesheet.

	# Print the stylesheet for the default theme.
	doc2go -highlight-print-css

	# Print the stylesheet for a specific theme.
	doc2go -highlight-print-css -highlight=plain

Both MODE and THEME are optional.
If only one is specified, it's assumed to be the THEME.
Add a trailing ':' to specify the MODE and use the default theme.

	-highlight=plain    # == auto:plain
	-highlight=classes: # == classes:` + _defaultStyle.Name + `
	-highlight=inline:plain
`

func init() { _helpTopics[HighlightHelp] = _highlightHelp }

// Write writes the help on this topic to the writer.
// If this topic is not known, an error is returned.
func (h Help) Write(w io.Writer) error {
	if h == NoHelp {
		return nil
	}

	if doc, ok := _helpTopics[h]; ok {
		_, err := io.WriteString(w, doc)
		return err
	}

	topics := make([]string, 0, len(_helpTopics))
	for h := range _helpTopics {
		topics = append(topics, string(h))
	}
	sort.Strings(topics)

	return fmt.Errorf("unknown help topic %q: valid values are %q", string(h), topics)
}

var _ flag.Getter = (*Help)(nil)

// Get returns the value of the Help.
// This is to comply with the [flag.Getter] interface.
func (h *Help) Get() any {
	return *h
}

// IsBoolFlag marks this as a boolean flag
// which allows it to be used without an argument.
func (*Help) IsBoolFlag() bool {
	return true
}

// String returns the name of this topic.
func (h Help) String() string {
	return string(h)
}

// Set receives a command line value.
func (h *Help) Set(s string) error {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "true" {
		s = "default"
	}
	*h = Help(s)
	return nil
}
