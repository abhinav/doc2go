package highlight

import chroma "github.com/alecthomas/chroma/v2"

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

	// TokenSpan is a span of code
	// that is highlighted with chroma.
	TokenSpan struct {
		Tokens []chroma.Token
	}

	// AnchorSpan renders as an addressable anchor point.
	AnchorSpan struct {
		Spans []Span
		ID    string
	}

	// LinkSpan renders as a link with a specific destination.
	LinkSpan struct {
		Spans []Span
		Dest  string
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
	_ Span = (*TokenSpan)(nil)
	_ Span = (*AnchorSpan)(nil)
	_ Span = (*LinkSpan)(nil)
	_ Span = (*ErrorSpan)(nil)
)

func (*TextSpan) span()   {}
func (*TokenSpan) span()  {}
func (*AnchorSpan) span() {}
func (*LinkSpan) span()   {}
func (*ErrorSpan) span()  {}
