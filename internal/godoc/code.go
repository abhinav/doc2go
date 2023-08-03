package godoc

import (
	"fmt"
	"go/doc/comment"

	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/highlight"
)

// CodeBuilder builds highlight.Code blocks,
// using the provided linker to resolve links to entities.
type CodeBuilder struct {
	DocLinkURL func(*comment.DocLink) string
	Lexer      highlight.Lexer
}

// Build builds a [highlight.Code] containing the provided source,
// annotated with the provided regions.
//
// Panics if regions are out of bounds in src,
// or an unknown region is encountered.
func (cb *CodeBuilder) Build(src []byte, regions []gosrc.Region) *highlight.Code {
	spansForText := cb.lexSpanner(src)
	if spansForText == nil {
		spansForText = func(start, end int) []highlight.Span {
			if start >= end {
				return nil
			}
			return []highlight.Span{&highlight.TextSpan{Text: src[start:end]}}
		}
	}

	var (
		lastOffset int
		spans      []highlight.Span
	)
	for _, r := range regions {
		spans = append(spans, spansForText(lastOffset, r.Offset)...)
		lastOffset = r.Offset + r.Length
		body := spansForText(r.Offset, lastOffset)
		switch l := r.Label.(type) {
		case *gosrc.DeclLabel:
			id := l.Name
			if len(l.Parent) > 0 {
				id = l.Parent + "." + id
			}

			spans = append(spans, &highlight.AnchorSpan{
				Spans: body,
				ID:    id,
			})

		case *gosrc.EntityRefLabel:
			dest := cb.DocLinkURL(&comment.DocLink{
				ImportPath: l.ImportPath,
				Name:       l.Name,
			})
			spans = append(spans, &highlight.LinkSpan{
				Spans: body,
				Dest:  dest,
			})

		case *gosrc.PackageRefLabel:
			dest := cb.DocLinkURL(&comment.DocLink{ImportPath: l.ImportPath})
			spans = append(spans, &highlight.LinkSpan{
				Spans: body,
				Dest:  dest,
			})

		default:
			panic(fmt.Sprintf("Unexpected label %T", l))
		}
	}
	spans = append(spans, spansForText(lastOffset, len(src))...)
	return &highlight.Code{Spans: spans}
}

func (cb *CodeBuilder) lexSpanner(src []byte) func(start, end int) []highlight.Span {
	// TODO: this could probably be in the highlight package.

	tokens, err := cb.Lexer.Lex(src)
	if err != nil || len(tokens) == 0 {
		return nil // TODO: Log this
	}

	tokIdx := highlight.NewTokenIndex(src, tokens)
	return func(start, end int) []highlight.Span {
		if start >= end {
			return nil
		}
		toks, lead, trail := tokIdx.Interval(start, end)
		var spans []highlight.Span
		if len(lead) > 0 {
			spans = append(spans, &highlight.TextSpan{Text: lead})
		}
		if len(toks) > 0 {
			spans = append(spans, &highlight.TokenSpan{Tokens: toks})
		}
		if len(trail) > 0 {
			spans = append(spans, &highlight.TextSpan{Text: trail})
		}
		return spans
	}
}
