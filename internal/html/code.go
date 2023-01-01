package html

import (
	"bytes"
	"fmt"
	"html/template"

	"go.abhg.dev/doc2go/internal/godoc"
)

func renderCode(code *godoc.Code) template.HTML {
	if code == nil {
		return ""
	}
	var buf bytes.Buffer
	for _, b := range code.Spans {
		switch b := b.(type) {
		case *godoc.TextSpan:
			template.HTMLEscape(&buf, b.Text)
		case *godoc.CommentSpan:
			buf.WriteString(`<span class="comment">`)
			template.HTMLEscape(&buf, b.Text)
			buf.WriteString(`</span>`)
		case *godoc.AnchorSpan:
			fmt.Fprintf(&buf, "<span id=%q>", b.ID)
			template.HTMLEscape(&buf, b.Text)
			buf.WriteString("</span>")
		case *godoc.LinkSpan:
			fmt.Fprintf(&buf, "<a href=%q>", b.Dest)
			template.HTMLEscape(&buf, b.Text)
			buf.WriteString("</a>")
		case *godoc.ErrorSpan:
			buf.WriteString("<strong>")
			template.HTMLEscape(&buf, []byte(b.Msg))
			buf.WriteString("</strong>")
			buf.WriteString("<pre><code>")
			template.HTMLEscape(&buf, []byte(b.Err.Error()))
			buf.WriteString("</code></pre>")
		default:
			panic(fmt.Sprintf("unrecognized node type %T", b))
		}
	}
	return template.HTML(buf.String())
}
