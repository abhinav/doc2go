package godoc

import (
	"fmt"
	"go/doc/comment"

	"go.abhg.dev/doc2go/internal/gosrc"
)

// Code is a code block comprised of multiple text nodes.
type Code struct {
	Spans []Span
}

type (
	// Span is a part of a code block.
	Span interface{ span() }

	// TextSpan is a span rendered as-is.
	TextSpan struct {
		Text []byte
	}

	// AnchorSpan renders as an addressable anchor point.
	AnchorSpan struct {
		Text []byte
		ID   string
	}

	// LinkSpan renders as a link with a specific destination.
	LinkSpan struct {
		Text []byte
		Dest string
	}

	// CommentSpan renders as slightly muted text.
	CommentSpan struct {
		Text []byte
	}

	// ErrorSpan is a special span
	// that represents a failure operation.
	//
	// This renders in HTML in a visible way
	// to avoid failing silently.
	ErrorSpan struct {
		Msg string
		Err error
	}
)

var (
	_ Span = (*TextSpan)(nil)
	_ Span = (*AnchorSpan)(nil)
	_ Span = (*LinkSpan)(nil)
	_ Span = (*ErrorSpan)(nil)
	_ Span = (*CommentSpan)(nil)
)

func (*TextSpan) span()    {}
func (*AnchorSpan) span()  {}
func (*LinkSpan) span()    {}
func (*ErrorSpan) span()   {}
func (*CommentSpan) span() {}

// CodeBuilder builds Code blocks,
// using the provided linker to resolve links to entities.
type CodeBuilder struct {
	DocLinkURL func(*comment.DocLink) string
}

// Build builds a [Code] containing the provided source,
// annotated with the provided regions.
//
// Panics if regions are out of bounds in src,
// or an unknown region is encountered.
func (cb *CodeBuilder) Build(src []byte, regions []gosrc.Region) *Code {
	var (
		spans      []Span
		lastOffset int
	)
	for _, r := range regions {
		if t := src[lastOffset:r.Offset]; len(t) > 0 {
			spans = append(spans, &TextSpan{Text: t})
		}

		lastOffset = r.Offset + r.Length
		body := src[r.Offset:lastOffset]
		switch l := r.Label.(type) {
		case *gosrc.DeclLabel:
			id := l.Name
			if len(l.Parent) > 0 {
				id = l.Parent + "." + id
			}

			spans = append(spans, &AnchorSpan{
				Text: body,
				ID:   id,
			})

		case *gosrc.EntityRefLabel:
			dest := cb.DocLinkURL(&comment.DocLink{
				ImportPath: l.ImportPath,
				Name:       l.Name,
			})
			spans = append(spans, &LinkSpan{
				Text: body,
				Dest: dest,
			})

		case *gosrc.PackageRefLabel:
			dest := cb.DocLinkURL(&comment.DocLink{ImportPath: l.ImportPath})
			spans = append(spans, &LinkSpan{
				Text: body,
				Dest: dest,
			})

		case *gosrc.CommentLabel:
			spans = append(spans, &CommentSpan{Text: body})

		default:
			panic(fmt.Sprintf("Unexpected label %T", l))
		}
	}
	if t := src[lastOffset:]; len(t) > 0 {
		spans = append(spans, &TextSpan{Text: t})
	}

	return &Code{Spans: spans}
}
