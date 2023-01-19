package highlight

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"sync"

	chroma "github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
)

// Highlighter turns [Code] into HTML.
type Highlighter struct {
	// Style used for syntax highlighting of code.
	Style *chroma.Style

	// UseClasses specifies whether the highlighter
	// uses inline 'style' attributes for highlighting,
	// or classes, assumign use of an appropriate style sheet.
	UseClasses bool

	once      sync.Once
	formatter *chromahtml.Formatter
}

func (h *Highlighter) init() {
	h.once.Do(func() {
		h.formatter = chromahtml.New(
			chromahtml.PreventSurroundingPre(true),
			chromahtml.WithClasses(h.UseClasses),
		)
	})
}

// WriteCSS writes the style classes for this highlighter to writer.
// If this highlighter is not using classes, WriteCSS is a no-op.
func (h *Highlighter) WriteCSS(w io.Writer) error {
	h.init()

	if !h.UseClasses {
		return nil
	}

	return h.formatter.WriteCSS(w, h.Style)
}

// Highlight renders the given code block into HTML.
func (h *Highlighter) Highlight(code *Code) string {
	h.init()

	if code == nil {
		return ""
	}

	r := codeRenderer{fmt: h.formatter, sty: h.Style}
	if h.UseClasses {
		fmt.Fprintf(&r, "<pre class=%q>", chroma.StandardTypes[chroma.PreWrapper])
	} else {
		style := chromahtml.StyleEntryToCSS(h.Style.Get(chroma.PreWrapper))
		fmt.Fprintf(&r, "<pre style=%q>", style)
	}
	r.RenderSpans(code.Spans)
	fmt.Fprint(&r, "</pre>")
	return r.String()
}

type codeRenderer struct {
	bytes.Buffer

	fmt chroma.Formatter
	sty *chroma.Style
}

func (r *codeRenderer) RenderSpans(spans []Span) {
	for _, span := range spans {
		r.RenderSpan(span)
	}
}

func (r *codeRenderer) RenderSpan(span Span) {
	switch b := span.(type) {
	case *TokenSpan:
		r.fmt.Format(r, r.sty, chroma.Literator(b.Tokens...))
	case *TextSpan:
		template.HTMLEscape(r, b.Text)
	case *AnchorSpan:
		fmt.Fprintf(r, "<span id=%q>", b.ID)
		r.RenderSpans(b.Spans)
		r.WriteString("</span>")
	case *LinkSpan:
		fmt.Fprintf(r, "<a href=%q>", b.Dest)
		r.RenderSpans(b.Spans)
		r.WriteString("</a>")
	case *ErrorSpan:
		r.WriteString("<strong>")
		template.HTMLEscape(r, []byte(b.Msg))
		r.WriteString("</strong>")
		r.WriteString("<pre><code>")
		template.HTMLEscape(r, []byte(b.Err.Error()))
		r.WriteString("</code></pre>")
	default:
		panic(fmt.Sprintf("unrecognized node type %T", b))
	}
}
