package highlight

import (
	"errors"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/stretchr/testify/assert"
)

func TestHighlighter_Highlight(t *testing.T) {
	t.Parallel()

	textSpan := func(s string) *TextSpan {
		return &TextSpan{Text: []byte(s)}
	}

	spans := func(spans ...Span) []Span {
		return spans
	}

	tests := []struct {
		desc string
		give Span
		want string
	}{
		{
			desc: "text",
			give: &TextSpan{
				Text: []byte("a < b"),
			},
			want: "a &lt; b",
		},
		{
			desc: "anchor",
			give: &AnchorSpan{
				ID:    "foo",
				Spans: spans(textSpan("bar & baz")),
			},
			want: `<span id="foo">bar &amp; baz</span>`,
		},
		{
			desc: "link",
			give: &LinkSpan{
				Spans: spans(textSpan("baz > qux")),
				Dest:  "https://example.com",
			},
			want: `<a href="https://example.com">baz &gt; qux</a>`,
		},
		{
			desc: "error",
			give: &ErrorSpan{
				Msg: "Something went wrong",
				Err: errors.New("great sadness"),
			},
			want: "<strong>Something went wrong</strong>" +
				"<pre><code>great sadness</code></pre>",
		},
		{
			desc: "highlight",
			give: &TokenSpan{
				Tokens: []chroma.Token{
					{Type: chroma.Comment, Value: "/* foo */"},
					{Type: chroma.Text, Value: "bar"},
				},
			},
			want: `<span class="c">/* foo */</span>bar`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			h := Highlighter{
				Style:      PlainStyle,
				UseClasses: true,
			}
			want := `<pre class="chroma">` + tt.want + "</pre>"
			got := h.Highlight(&Code{
				Spans: []Span{tt.give},
			})
			assert.Equal(t, want, string(got))
		})
	}

	t.Run("unknown", func(t *testing.T) {
		type unknownSpan struct{ Span }

		assert.Panics(t, func() {
			h := Highlighter{
				Style: PlainStyle,
			}
			h.Highlight(&Code{
				Spans: []Span{unknownSpan{}},
			})
		})
	})
}

func TestHighlighter_Highlight_noClasses(t *testing.T) {
	t.Parallel()

	h := Highlighter{Style: PlainStyle}
	want := `<pre style="background-color: #eeeeee">` +
		`<span style="color:#666">/* foo */</span>bar` +
		`</pre>`
	got := h.Highlight(&Code{
		Spans: []Span{
			&TokenSpan{
				Tokens: []chroma.Token{
					{Type: chroma.Comment, Value: "/* foo */"},
					{Type: chroma.Text, Value: "bar"},
				},
			},
		},
	})
	assert.Equal(t, want, string(got))
}
