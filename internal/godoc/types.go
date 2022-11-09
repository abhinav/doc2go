package godoc

import (
	"go/doc/comment"

	"go.abhg.dev/doc2go/internal/code"
)

// Package holds documentation for a single Go package.
type Package struct {
	Name string
	Doc  *comment.Doc // package-level documentation

	ImportPath string
	Synopsis   string

	Constants []*Value
	Variables []*Value
	Types     []*Type
	Functions []*Function
}

// Value is a top-level constant or variable or a group fo them
// declared in a package.
type Value struct {
	Names []string
	Doc   *comment.Doc
	Decl  *code.Block
}

// Type is a single top-level type.
type Type struct {
	Name string
	Doc  *comment.Doc
	Decl *code.Block

	// Constants, variables, functions, and methods
	// associated with this type.
	Constants, Variables []*Value
	Functions, Methods   []*Function
}

// Function is a top-level function or method.
type Function struct {
	Name string
	Doc  *comment.Doc
	Decl *code.Block
	Recv string // only set for methods
}
