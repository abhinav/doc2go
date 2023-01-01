package html

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.abhg.dev/doc2go/internal/godoc"
)

func TestRenderCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give godoc.Span
		want string
	}{
		{
			desc: "text",
			give: &godoc.TextSpan{
				Text: []byte("a < b"),
			},
			want: "a &lt; b",
		},
		{
			desc: "comment",
			give: &godoc.CommentSpan{
				Text: []byte("a < b"),
			},
			want: `<span class="comment">a &lt; b</span>`,
		},
		{
			desc: "anchor",
			give: &godoc.AnchorSpan{
				ID:   "foo",
				Text: []byte("bar & baz"),
			},
			want: `<span id="foo">bar &amp; baz</span>`,
		},
		{
			desc: "link",
			give: &godoc.LinkSpan{
				Text: []byte("baz > qux"),
				Dest: "https://example.com",
			},
			want: `<a href="https://example.com">baz &gt; qux</a>`,
		},
		{
			desc: "error",
			give: &godoc.ErrorSpan{
				Msg: "Something went wrong",
				Err: errors.New("great sadness"),
			},
			want: "<strong>Something went wrong</strong>" +
				"<pre><code>great sadness</code></pre>",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got := renderCode(&godoc.Code{
				Spans: []godoc.Span{tt.give},
			})
			assert.Equal(t, tt.want, string(got))
		})
	}

	t.Run("unknown", func(t *testing.T) {
		type unknownSpan struct{ godoc.Span }

		assert.Panics(t, func() {
			renderCode(&godoc.Code{
				Spans: []godoc.Span{unknownSpan{}},
			})
		})
	})
}
