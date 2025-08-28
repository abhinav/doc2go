package highlight

import (
	"fmt"
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
				{Type: chroma.Whitespace, Value: " "},
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
				{Type: chroma.Whitespace, Value: " "},
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
				{Type: chroma.Whitespace, Value: " "},
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
				{Type: chroma.Whitespace, Value: "\n"},
				{Type: chroma.Punctuation, Value: "}"},
				{Type: chroma.Whitespace, Value: "\n"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			tokens, lead, trail := tidx.Interval(tt.start, tt.end)
			assert.Equal(t, tt.tokens, tokens)
			assert.Equal(t, tt.lead, string(lead))
			assert.Equal(t, tt.trail, string(trail))
		})
	}
}

// TestTokenIndexNoPanic tests that TokenIndex.Interval does not panic
// for various edge cases, including the condition that probably caused
// https://github.com/abhinav/doc2go/issues/290
func TestTokenIndexNoPanic(t *testing.T) {
	t.Parallel()

	tests := []string{
		`package main`,

		`package main

import "fmt"

func main() {
	fmt.Println("hello")
}`,
		`// Comment at start
package main
/* multi
   line
   comment */
func test() {}`,

		`package main; import "fmt"; func main() { fmt.Println("compact") }`,

		"package main\n\n\n\nfunc main() {\n\t\t\n\tprintln(\"lots of whitespace\")\n\n}",

		`package main

import (
	"fmt"
	"strings"
)

func main() {
	s := "string with\ttabs and\nnewlines"
	fmt.Println(strings.TrimSpace(s))
}`,
	}

	for i, src := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()

			tokens, err := GoLexer.Lex([]byte(src))
			require.NoError(t, err)

			tidx := NewTokenIndex([]byte(src), tokens)

			srcLen := len(src)
			for start := 0; start <= srcLen; start++ {
				for end := start; end <= srcLen; end++ {
					require.NotPanics(t, func() {
						tidx.Interval(start, end)
					}, "start=%d end=%d", start, end)
				}
			}
		})
	}
}
