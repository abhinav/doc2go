package highlight

import (
	"testing"

	chroma "github.com/alecthomas/chroma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenIndex(t *testing.T) {
	t.Parallel()

	src := []byte("func foo() (int, error) {\n\treturn 42, nil\n}\n")
	tokens, err := GoLexer.Lex(src)
	require.NoError(t, err)

	tidx := NewTokenIndex(src, tokens)

	tests := []struct {
		desc        string
		start, end  int
		tokens      []chroma.Token
		lead, trail string
	}{
		{
			desc: "exact boundaries",
			// func foo()
			start: 0,
			end:   10,
			tokens: []chroma.Token{
				{Type: chroma.KeywordDeclaration, Value: "func"},
				{Type: chroma.Text, Value: " "},
				{Type: chroma.NameFunction, Value: "foo"},
				{Type: chroma.Punctuation, Value: "()"},
			},
		},
		{
			desc: "leading text",
			// unc foo()
			start: 1,
			end:   10,
			lead:  "unc",
			tokens: []chroma.Token{
				{Type: chroma.Text, Value: " "},
				{Type: chroma.NameFunction, Value: "foo"},
				{Type: chroma.Punctuation, Value: "()"},
			},
		},
		{
			desc: "trailing text",
			// func fo
			start: 0,
			end:   7,
			tokens: []chroma.Token{
				{Type: chroma.KeywordDeclaration, Value: "func"},
				{Type: chroma.Text, Value: " "},
			},
			trail: "fo",
		},
		{
			desc:  "start out of range",
			start: 75,
			end:   100,
		},
		{
			desc: "hit end of src",
			// , nil\n}\n
			start: 38,
			end:   len(src),
			tokens: []chroma.Token{
				{Type: chroma.KeywordConstant, Value: "nil"},
				{Type: chroma.Text, Value: "\n"},
				{Type: chroma.Punctuation, Value: "}"},
				{Type: chroma.Text, Value: "\n"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			tokens, lead, trail := tidx.Interval(tt.start, tt.end)
			assert.Equal(t, tt.tokens, tokens)
			assert.Equal(t, tt.lead, string(lead))
			assert.Equal(t, tt.trail, string(trail))
		})
	}
}
