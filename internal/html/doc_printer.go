package html

import "go/doc/comment"

// DocPrinter formats godoc comments as HTML.
type DocPrinter interface {
	HTML(*comment.Doc) []byte
	WithHeadingLevel(int) DocPrinter
}

// CommentDocPrinter is a DocPrinter
// built from a [comment.Printer].
type CommentDocPrinter struct{ comment.Printer }

var _ DocPrinter = (*CommentDocPrinter)(nil)

// WithHeadingLevel returns a copy of this DocPrinter
// that will generate headers at the specified level.
func (dp *CommentDocPrinter) WithHeadingLevel(lvl int) DocPrinter {
	out := *dp
	out.HeadingLevel = lvl
	return &out
}
