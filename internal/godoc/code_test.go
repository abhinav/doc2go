package godoc

import (
	"errors"
	"go/doc/comment"
	"testing"

	chroma "github.com/alecthomas/chroma/v2"
	"github.com/stretchr/testify/assert"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/highlight"
)

func TestCodeBuilder(t *testing.T) {
	t.Parallel()

	textSpan := func(text string) *highlight.TextSpan {
		return &highlight.TextSpan{Text: []byte(text)}
	}

	anchorSpan := func(id string, spans ...highlight.Span) *highlight.AnchorSpan {
		return &highlight.AnchorSpan{ID: id, Spans: spans}
	}

	linkSpan := func(dest string, spans ...highlight.Span) *highlight.LinkSpan {
		return &highlight.LinkSpan{Dest: dest, Spans: spans}
	}

	type tokens []chroma.Token
	tokenSpan := func(toks tokens) *highlight.TokenSpan {
		return &highlight.TokenSpan{Tokens: toks}
	}
	singleTokenSpan := func(typ chroma.TokenType, value string) *highlight.TokenSpan {
		return &highlight.TokenSpan{Tokens: tokens{{Type: typ, Value: value}}}
	}

	spans := func(spans ...highlight.Span) []highlight.Span {
		return spans
	}

	tests := []struct {
		desc    string
		src     string
		regions []gosrc.Region
		tokens  []chroma.Token
		want    []highlight.Span
	}{
		{
			desc: "no regions",
			src:  "func foo() {}",
			want: spans(
				textSpan("func foo() {}"),
			),
		},
		{
			desc: "field decl and entity ref",
			src:  "\tName string",
			regions: []gosrc.Region{
				{
					Label: &gosrc.DeclLabel{
						Parent: "User",
						Name:   "Name",
					},
					Offset: 1,
					Length: 4,
				},
				{
					Label: &gosrc.EntityRefLabel{
						ImportPath: gosrc.Builtin,
						Name:       "string",
					},
					Offset: 6,
					Length: 6,
				},
			},
			want: spans(
				textSpan("\t"),
				anchorSpan("User.Name", textSpan("Name")),
				textSpan(" "),
				linkSpan("https://example.com/builtin#string", textSpan("string")),
			),
		},
		{
			desc: "entity and package ref",
			src:  "{ W io.Writer }",
			regions: []gosrc.Region{
				{
					Label: &gosrc.DeclLabel{
						Parent: "Logger",
						Name:   "W",
					},
					Offset: 2,
					Length: 1,
				},
				{
					Label:  &gosrc.PackageRefLabel{ImportPath: "io"},
					Offset: 4,
					Length: 2,
				},
				{
					Label: &gosrc.EntityRefLabel{
						ImportPath: "io",
						Name:       "Writer",
					},
					Offset: 7,
					Length: 6,
				},
			},
			want: spans(
				textSpan("{ "),
				anchorSpan("Logger.W", textSpan("W")),
				textSpan(" "),
				linkSpan("https://example.com/io", textSpan("io")),
				textSpan("."),
				linkSpan("https://example.com/io#Writer", textSpan("Writer")),
				textSpan(" }"),
			),
		},
		{
			desc: "no regions full highlight",
			src:  "func foo() {}",
			tokens: []chroma.Token{
				{Type: chroma.Text, Value: "func foo() {}"},
			},
			want: spans(
				tokenSpan(tokens{
					{Type: chroma.Text, Value: "func foo() {}"},
				}),
			),
		},
		{
			desc: "highlight sections",
			src:  "\tName string",
			regions: []gosrc.Region{
				{
					Label: &gosrc.DeclLabel{
						Parent: "User",
						Name:   "Name",
					},
					Offset: 1,
					Length: 4,
				},
				{
					Label: &gosrc.EntityRefLabel{
						ImportPath: gosrc.Builtin,
						Name:       "string",
					},
					Offset: 6,
					Length: 6,
				},
			},
			tokens: []chroma.Token{
				{Type: chroma.Text, Value: "\t"},
				{Type: chroma.NameProperty, Value: "Name"},
				{Type: chroma.Text, Value: " "},
				{Type: chroma.NameBuiltin, Value: "string"},
			},
			want: spans(
				singleTokenSpan(chroma.Text, "\t"),
				anchorSpan("User.Name", singleTokenSpan(chroma.NameProperty, "Name")),
				singleTokenSpan(chroma.Text, " "),
				linkSpan("https://example.com/builtin#string", singleTokenSpan(chroma.NameBuiltin, "string")),
			),
		},
		{
			desc: "highlight lead/trail",
			src:  "{ Writer ",
			regions: []gosrc.Region{
				{
					// For some reason, our label is
					// only on a part of the item.
					Label: &gosrc.DeclLabel{
						Parent: "Logger",
						Name:   "Wri",
					},
					Offset: 2,
					Length: 3,
				},
			},
			tokens: []chroma.Token{
				{Type: chroma.Punctuation, Value: "{"},
				{Type: chroma.Text, Value: " "},
				{Type: chroma.NameProperty, Value: "Writer"},
				{Type: chroma.Text, Value: " "},
			},
			want: spans(
				tokenSpan(tokens{
					{Type: chroma.Punctuation, Value: "{"},
					{Type: chroma.Text, Value: " "},
				}),
				anchorSpan("Logger.Wri", textSpan("Wri")),
				textSpan("ter"),
				singleTokenSpan(chroma.Text, " "),
			),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			cb := CodeBuilder{
				Lexer: &stubLexer{
					Result: tt.tokens,
				},
				DocLinkURL: func(dl *comment.DocLink) string {
					return dl.DefaultURL("https://example.com")
				},
			}

			got := cb.Build([]byte(tt.src), tt.regions)
			assert.Equal(t, tt.want, got.Spans)
		})
	}

	t.Run("unexpected label", func(t *testing.T) {
		cb := CodeBuilder{
			Lexer: &stubLexer{
				Err: errors.New("great sadness"),
			},
			DocLinkURL: func(dl *comment.DocLink) string {
				return dl.DefaultURL("https://example.com")
			},
		}

		type invalidLabel struct{ gosrc.Label }

		assert.Panics(t, func() {
			cb.Build([]byte("foo bar"), []gosrc.Region{
				{
					Offset: 0,
					Length: 5,
					Label:  invalidLabel{},
				},
			})
		})
	})
}
