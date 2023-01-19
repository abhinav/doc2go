package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Help is doc2go's -h/-help flag.
// It supports retrieving help on various topics by passing in a parameter.
type Help string

var (
	//go:embed help/default.txt
	_defaultHelp string

	//go:embed help/frontmatter.txt
	_frontmatterHelp string

	//go:embed help/pkg-doc.txt
	_packageDocHelp string

	//go:embed help/highlight.txt
	_highlightHelp string

	//go:embed help/config.txt
	_configHelp string

	_usageHelp = firstLineOf(_defaultHelp)

	_helpTopics = map[Help]string{
		"config":      _configHelp,
		"default":     _defaultHelp,
		"frontmatter": _frontmatterHelp,
		"highlight":   _highlightHelp,
		"pkg-doc":     _packageDocHelp,
		"usage":       _usageHelp,
	}
)

func firstLineOf(s string) string {
	if idx := strings.IndexRune(s, '\n'); idx >= 0 {
		s = s[:idx+1]
	}
	return s
}

// Write writes the help on this topic to the writer.
// If this topic is not known, an error is returned.
func (h Help) Write(w io.Writer) error {
	if len(h) == 0 {
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
