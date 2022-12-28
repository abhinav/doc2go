package godoc

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
