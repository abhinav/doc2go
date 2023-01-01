package godoc

import (
	"go/doc/comment"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.abhg.dev/doc2go/internal/gosrc"
)

func TestCodeBuilder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc    string
		src     string
		regions []gosrc.Region
		want    []Span
	}{
		{
			desc: "no regions",
			src:  "func foo() {}",
			want: []Span{
				&TextSpan{Text: []byte("func foo() {}")},
			},
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
			want: []Span{
				&TextSpan{Text: []byte("\t")},
				&AnchorSpan{Text: []byte("Name"), ID: "User.Name"},
				&TextSpan{Text: []byte(" ")},
				&LinkSpan{
					Text: []byte("string"),
					Dest: "https://example.com/builtin#string",
				},
			},
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
			want: []Span{
				&TextSpan{Text: []byte("{ ")},
				&AnchorSpan{Text: []byte("W"), ID: "Logger.W"},
				&TextSpan{Text: []byte(" ")},
				&LinkSpan{
					Text: []byte("io"),
					Dest: "https://example.com/io",
				},
				&TextSpan{Text: []byte(".")},
				&LinkSpan{
					Text: []byte("Writer"),
					Dest: "https://example.com/io#Writer",
				},
				&TextSpan{Text: []byte(" }")},
			},
		},
		{
			desc: "comment",
			src:  "func Foo(/* foo */ string)",
			regions: []gosrc.Region{
				{
					Offset: 9,
					Length: 9,
					Label:  &gosrc.CommentLabel{},
				},
			},
			want: []Span{
				&TextSpan{Text: []byte("func Foo(")},
				&CommentSpan{Text: []byte("/* foo */")},
				&TextSpan{Text: []byte(" string)")},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			cb := CodeBuilder{
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
