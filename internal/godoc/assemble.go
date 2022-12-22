// Package godoc provides the means of converting parsed Go source information
// into a documentation subset of it.
// This information is neessary to render documentation for a package.
package godoc

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/printer"
	"go/token"

	"go.abhg.dev/doc2go/internal/code"
	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/slices"
)

// Assembler assembles a [Package] from a [go/doc.Package].
type Assembler struct {
	// TODO:
	// link resolvers go here

	// Reference to doc.NewFromFiles.
	//
	// May be overridden during tests.
	docNewFromFiles func(*token.FileSet, []*ast.File, string, ...any) (*doc.Package, error)
}

// Assemble runs the assembler on the given doc.Package.
func (a *Assembler) Assemble(bpkg *gosrc.Package) (*Package, error) {
	docNewFromFiles := doc.NewFromFiles
	if a.docNewFromFiles != nil {
		docNewFromFiles = a.docNewFromFiles
	}

	dpkg, err := docNewFromFiles(bpkg.Fset, bpkg.Syntax, bpkg.ImportPath)
	if err != nil {
		return nil, fmt.Errorf("assemble documentation: %w", err)
	}

	return (&assembly{
		fset:   bpkg.Fset,
		cparse: dpkg.Parser(),
	}).pkg(dpkg), nil
}

type assembly struct {
	fset   *token.FileSet
	cparse *comment.Parser
}

func (as *assembly) doc(doc string) *comment.Doc {
	return as.cparse.Parse(doc)
}

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

func (as *assembly) pkg(dpkg *doc.Package) *Package {
	return &Package{
		Name:       dpkg.Name,
		Doc:        as.doc(dpkg.Doc),
		ImportPath: dpkg.ImportPath,
		Synopsis:   dpkg.Synopsis(dpkg.Doc),
		Constants:  slices.Transform(dpkg.Consts, as.val),
		Variables:  slices.Transform(dpkg.Vars, as.val),
		Types:      slices.Transform(dpkg.Types, as.typ),
		Functions:  slices.Transform(dpkg.Funcs, as.fun),
	}
}

// Value is a top-level constant or variable or a group fo them
// declared in a package.
type Value struct {
	Names []string
	Doc   *comment.Doc
	Decl  *code.Block
}

func (as *assembly) val(dval *doc.Value) *Value {
	return &Value{
		Names: dval.Names,
		Doc:   as.doc(dval.Doc),
		Decl:  as.decl(dval.Decl),
	}
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

func (as *assembly) typ(dtyp *doc.Type) *Type {
	return &Type{
		Name:      dtyp.Name,
		Doc:       as.doc(dtyp.Doc),
		Decl:      as.decl(dtyp.Decl),
		Constants: slices.Transform(dtyp.Consts, as.val),
		Variables: slices.Transform(dtyp.Vars, as.val),
		Functions: slices.Transform(dtyp.Funcs, as.fun),
		Methods:   slices.Transform(dtyp.Methods, as.fun),
	}
}

// Function is a top-level function or method.
type Function struct {
	Name      string
	Doc       *comment.Doc
	Decl      *code.Block
	ShortDecl string
	Recv      string // only set for methods
}

func (as *assembly) fun(dfun *doc.Func) *Function {
	return &Function{
		Name:      dfun.Name,
		Doc:       as.doc(dfun.Doc),
		Decl:      as.decl(dfun.Decl),
		ShortDecl: as.shortDecl(dfun.Decl),
		Recv:      dfun.Recv,
	}
}

func (as *assembly) decl(decl ast.Decl) *code.Block {
	var buf bytes.Buffer
	err := (&printer.Config{
		Mode:     printer.UseSpaces,
		Tabwidth: 8,
	}).Fprint(&buf, as.fset, decl)
	if err != nil {
		return &code.Block{
			Nodes: []code.Node{
				&code.TextNode{
					Text: []byte(err.Error()),
				},
			},
		}
	}

	// TODO: links
	return &code.Block{
		Nodes: []code.Node{
			&code.TextNode{
				Text: buf.Bytes(),
			},
		},
	}
}

func (as *assembly) shortDecl(decl ast.Decl) string {
	return OneLineNodeDepth(as.fset, decl, 0)
}
