// Package godoc provides the means of converting parsed Go source information
// into a documentation subset of it.
// This information is neessary to render documentation for a package.
package godoc

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/token"

	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/slices"
)

// Linker generates links to the documentation for a specific package or
// entity.
type Linker interface {
	DocLinkURL(fromPkg string, link *comment.DocLink) string
}

// Assembler assembles a [Package] from a [go/doc.Package].
type Assembler struct {
	Linker Linker

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
		fmt:        gosrc.NewDeclFormatter(bpkg),
		fset:       bpkg.Fset,
		cparse:     dpkg.Parser(),
		linker:     a.Linker,
		importPath: bpkg.ImportPath,
	}).pkg(dpkg), nil
}

type assembly struct {
	fmt        *gosrc.DeclFormatter
	fset       *token.FileSet
	cparse     *comment.Parser
	linker     Linker
	importPath string
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
	Decl  *Code
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
	Decl *Code

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
		Methods: slices.Transform(dtyp.Methods, func(f *doc.Func) *Function {
			fn := as.fun(f)
			fn.RecvType = dtyp.Name
			return fn
		}),
	}
}

// Function is a top-level function or method.
type Function struct {
	Name      string
	Doc       *comment.Doc
	Decl      *Code
	ShortDecl string
	Recv      string // only set for methods
	RecvType  string // name of the receiver type without '*'
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

func (as *assembly) decl(decl ast.Decl) *Code {
	// TODO: this can probably be extracted

	src, regions, err := as.fmt.FormatDecl(decl)
	if err != nil {
		return &Code{
			Spans: []Span{
				&ErrorSpan{
					Err: err,
					Msg: "Could not format declaration",
				},
			},
		}
	}

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
			dest := as.linker.DocLinkURL(as.importPath, &comment.DocLink{
				ImportPath: l.ImportPath,
				Name:       l.Name,
			})
			spans = append(spans, &LinkSpan{
				Text: body,
				Dest: dest,
			})

		case *gosrc.PackageRefLabel:
			dest := as.linker.DocLinkURL(as.importPath, &comment.DocLink{
				ImportPath: l.ImportPath,
			})
			spans = append(spans, &LinkSpan{
				Text: body,
				Dest: dest,
			})

		default:
			panic(fmt.Sprintf("Unexpected label %T", l))
		}
	}
	if t := src[lastOffset:]; len(t) > 0 {
		spans = append(spans, &TextSpan{Text: t})
	}

	return &Code{Spans: spans}
}

func (as *assembly) shortDecl(decl ast.Decl) string {
	return OneLineNodeDepth(as.fset, decl, 0)
}
