// Package code provides facilities for representing and rendering code blocks.
package code

import (
	"bytes"
	"fmt"
)

// Block is a code block comprised of multiple text nodes.
type Block struct {
	Nodes []Node
}

// Plain renders the given block as plain text.
func (blk *Block) Plain() string {
	var buf bytes.Buffer
	for _, b := range blk.Nodes {
		switch b := b.(type) {
		case *TextNode:
			buf.Write(b.Text)
		case *AnchorNode:
			buf.Write(b.Text)
		case *LinkNode:
			buf.Write(b.Text)
		default:
			panic(fmt.Sprintf("unrecognized node type %T", b))
		}
	}
	return buf.String()
}

type (
	// Node is a part of a code block.
	Node interface{ node() }

	// TextNode is a node representing plain text
	// in a code block.
	TextNode struct {
		Text []byte
	}

	// AnchorNode generates an addressable anchor
	// that we can use for permalinks.
	AnchorNode struct {
		Text []byte
		ID   string
	}

	// LinkNode generates a clickable link.
	LinkNode struct {
		Text []byte
		Dest string
	}
)

var (
	_ Node = (*TextNode)(nil)
	_ Node = (*AnchorNode)(nil)
	_ Node = (*LinkNode)(nil)
)

func (*TextNode) node()   {}
func (*AnchorNode) node() {}
func (*LinkNode) node()   {}
