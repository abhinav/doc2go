// Package godoc provides the means of converting parsed Go source information
// into a documentation subset of it.
// This information is neessary to render documentation for a package.
package godoc

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/format"
	"go/printer"
	"go/token"
	"io"
	"log"
	"path"
	"slices"

	"go.abhg.dev/doc2go/internal/gosrc"
	"go.abhg.dev/doc2go/internal/highlight"
	"go.abhg.dev/doc2go/internal/sliceutil"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Linker generates links to the documentation for a specific package or
// entity.
type Linker interface {
	DocLinkURL(fromPkg string, link *comment.DocLink) string
}

// DeclFormatter formats an AST declaration for rendering in documentation.
type DeclFormatter interface {
	FormatDecl(ast.Decl) (src []byte, regions []gosrc.Region, err error)
}

var _ DeclFormatter = (*gosrc.DeclFormatter)(nil)

// newDefaultDeclFormatter builds a DeclFormatter based on
// [gosrc.DeclFormatter].
func newDefaultDeclFormatter(pkg *gosrc.Package) DeclFormatter {
	return gosrc.NewDeclFormatter(pkg.Fset, pkg.TopLevelDecls)
}

// Assembler assembles a [Package] from a [go/doc.Package].
type Assembler struct {
	Linker Linker
	Logger *log.Logger // optional

	// Lexer used to highlight code blocks.
	Lexer highlight.Lexer

	// newDeclFormatter builds a DeclFormatter for the given package.
	//
	// This may be overriden from tests.
	newDeclFormatter func(*gosrc.Package) DeclFormatter
}

// Assemble runs the assembler on the given doc.Package.
func (a *Assembler) Assemble(bpkg *gosrc.Package) (*Package, error) {
	allSyntaxes := make([]*ast.File, len(bpkg.Syntax)+len(bpkg.TestSyntax))
	copy(allSyntaxes, bpkg.Syntax)
	copy(allSyntaxes[len(bpkg.Syntax):], bpkg.TestSyntax)

	dpkg, err := doc.NewFromFiles(bpkg.Fset, allSyntaxes, bpkg.ImportPath)
	if err != nil {
		return nil, fmt.Errorf("assemble documentation: %w", err)
	}

	newDeclFormatter := newDefaultDeclFormatter
	if a.newDeclFormatter != nil {
		newDeclFormatter = a.newDeclFormatter
	}

	logger := a.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	return (&assembly{
		fmt:        newDeclFormatter(bpkg),
		fset:       bpkg.Fset,
		cparse:     dpkg.Parser(),
		linker:     a.Linker,
		lexer:      a.Lexer,
		importPath: bpkg.ImportPath,
		logger:     logger,
	}).pkg(dpkg), nil
}

type assembly struct {
	fmt        DeclFormatter
	fset       *token.FileSet
	cparse     *comment.Parser
	linker     Linker
	importPath string
	lexer      highlight.Lexer
	logger     *log.Logger

	allExamples []*Example
}

func (as *assembly) logf(format string, args ...interface{}) {
	format = "[%v] " + format
	as.logger.Printf(format, append([]any{as.importPath}, args...)...)
}

func (as *assembly) doc(doc string) *comment.Doc {
	if len(doc) == 0 {
		return nil
	}
	return as.cparse.Parse(doc)
}

// Package holds documentation for a single Go package.
type Package struct {
	Name string
	Doc  *comment.Doc // package-level documentation

	// Empty if the package isn't a binary.
	BinName string

	ImportPath string
	Import     *highlight.Code // code form of import path
	Synopsis   string

	Constants []*Value
	Variables []*Value
	Types     []*Type
	Functions []*Function
	Examples  []*Example

	// All examples in the package and its children.
	AllExamples []*Example
}

func (as *assembly) pkg(dpkg *doc.Package) *Package {
	var binName string
	if dpkg.Name == "main" {
		binName = path.Base(dpkg.ImportPath)
	}

	pkg := &Package{
		Name:       dpkg.Name,
		Doc:        as.doc(dpkg.Doc),
		BinName:    binName,
		ImportPath: dpkg.ImportPath,
		Import:     as.importFor(dpkg.Name, dpkg.ImportPath),
		Synopsis:   dpkg.Synopsis(dpkg.Doc),
		Constants:  sliceutil.Transform(dpkg.Consts, as.val),
		Variables:  sliceutil.Transform(dpkg.Vars, as.val),
		Types:      sliceutil.Transform(dpkg.Types, as.typ),
		Functions:  as.funs("", dpkg.Funcs),
		Examples:   as.egs(ExampleParent{}, dpkg.Examples),
	}

	// Sort examples by parent, then by suffix.
	slices.SortFunc(as.allExamples, func(i, j *Example) int {
		if x := cmp.Compare(i.Parent.String(), j.Parent.String()); x != 0 {
			return x
		}
		return cmp.Compare(i.Suffix, j.Suffix)
	})
	pkg.AllExamples = as.allExamples

	return pkg
}

func (as *assembly) importFor(name, imp string) *highlight.Code {
	var buff bytes.Buffer
	if path.Base(imp) != name && name != "main" {
		fmt.Fprintf(&buff, "import %v %q", name, imp)
	} else {
		fmt.Fprintf(&buff, "import %q", imp)
	}

	tokens, err := as.lexer.Lex(buff.Bytes())
	if err != nil {
		// If the syntax highlighter fails,
		// show the statement without highlighting.
		as.logf("Error highlighting import statement: %v", err)
		return &highlight.Code{
			Spans: []highlight.Span{
				&highlight.TextSpan{Text: buff.Bytes()},
			},
		}
	}

	return &highlight.Code{
		Spans: []highlight.Span{
			&highlight.TokenSpan{Tokens: tokens},
		},
	}
}

// Value is a top-level constant or variable or a group fo them
// declared in a package.
type Value struct {
	Names []string
	Doc   *comment.Doc
	Decl  *highlight.Code
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
	Decl *highlight.Code

	// Constants, variables, functions, and methods
	// associated with this type.
	Constants, Variables []*Value
	Functions, Methods   []*Function

	Examples []*Example
}

func (as *assembly) typ(dtyp *doc.Type) *Type {
	return &Type{
		Name:      dtyp.Name,
		Doc:       as.doc(dtyp.Doc),
		Decl:      as.decl(dtyp.Decl),
		Constants: sliceutil.Transform(dtyp.Consts, as.val),
		Variables: sliceutil.Transform(dtyp.Vars, as.val),
		Functions: as.funs("" /* recv */, dtyp.Funcs),
		Methods:   as.funs(dtyp.Name, dtyp.Methods),
		Examples:  as.egs(ExampleParent{Name: dtyp.Name}, dtyp.Examples),
	}
}

// Function is a top-level function or method.
type Function struct {
	Name      string
	Doc       *comment.Doc
	Decl      *highlight.Code
	ShortDecl string
	Recv      string // only set for methods
	RecvType  string // name of the receiver type without '*'

	Examples []*Example
}

// parent is the name of the receiver for this function,
// or an empty string if it's a top-level function.
func (as *assembly) funs(parent string, dfun []*doc.Func) []*Function {
	if len(dfun) == 0 {
		return nil
	}

	funs := make([]*Function, len(dfun))
	for i, f := range dfun {
		funs[i] = as.fun(parent, f)
	}
	return funs
}

func (as *assembly) fun(parent string, dfun *doc.Func) *Function {
	return &Function{
		Name:      dfun.Name,
		Doc:       as.doc(dfun.Doc),
		Decl:      as.decl(dfun.Decl),
		ShortDecl: as.shortDecl(dfun.Decl),
		Recv:      dfun.Recv,
		RecvType:  parent,
		Examples: as.egs(ExampleParent{
			Recv: parent,
			Name: dfun.Name,
		}, dfun.Examples),
	}
}

func (as *assembly) decl(decl ast.Decl) *highlight.Code {
	src, regions, err := as.fmt.FormatDecl(decl)
	if err != nil {
		return &highlight.Code{
			Spans: []highlight.Span{
				&highlight.ErrorSpan{
					Err: err,
					Msg: "Could not format declaration",
				},
			},
		}
	}

	return (&CodeBuilder{
		Lexer: as.lexer,
		DocLinkURL: func(link *comment.DocLink) string {
			return as.linker.DocLinkURL(as.importPath, link)
		},
	}).Build(src, regions)
}

func (as *assembly) shortDecl(decl ast.Decl) string {
	return OneLineNodeDepth(as.fset, decl, 0)
}

// ExampleParent is a parent of a code example.
//
// Valid configurations for it are:
//
//   - package: Recv and Name are empty
//   - function or type: Recv is empty, Name is set
//   - method: Recv and Name are set
type ExampleParent struct {
	Recv string
	Name string
}

func (p ExampleParent) String() string {
	if p.Name == "" {
		return "package"
	}
	if p.Recv == "" {
		return p.Name
	}
	return p.Recv + "." + p.Name
}

// Example is a testable example found in a _test.go file.
// Each example is associated with
// either a package, a function, a type, or a method.
type Example struct {
	// Parent is the name of the entity this example is for.
	//
	// If Parent is empty, this is a package example.
	Parent ExampleParent

	// Suffix is the description of this example
	// following the entity association.
	//
	// This may be empty.
	Suffix string

	// Doc is the documentation for this example.
	Doc *comment.Doc

	// Code is the lexically analyzed code for this example,
	// ready to be syntax-highlighted.
	Code *highlight.Code

	// Output is the output expected from this example, if any.
	Output string
}

// Assembles a list of examples owned by the same parent.
func (as *assembly) egs(parent ExampleParent, dexs []*doc.Example) []*Example {
	if len(dexs) == 0 {
		return nil
	}

	exs := make([]*Example, len(dexs))
	for i, dex := range dexs {
		exs[i] = as.eg(parent, dex)
	}
	return exs
}

func (as *assembly) eg(parent ExampleParent, dex *doc.Example) (ex *Example) {
	defer func() {
		as.allExamples = append(as.allExamples, ex)
	}()

	code, err := as.egCode(dex)
	if err != nil {
		as.logf("Could not format example defined in %v: %v", as.fset.Position(dex.Code.Pos()), err)
		code = &highlight.Code{
			Spans: []highlight.Span{
				&highlight.ErrorSpan{
					Err: err,
					Msg: "Could not format example",
				},
			},
		}
	}

	suffix := cases.Title(language.English, cases.NoLower).String(dex.Suffix)
	return &Example{
		Parent: parent,
		Suffix: suffix,
		Code:   code,
		Doc:    as.doc(dex.Doc),
		Output: dex.Output,
	}
}

func (as *assembly) egCode(dex *doc.Example) (*highlight.Code, error) {
	var n any
	if dex.Play != nil {
		n = dex.Play
	} else {
		n = &printer.CommentedNode{
			Node:     dex.Code,
			Comments: dex.Comments,
		}
	}

	var buff bytes.Buffer
	if err := format.Node(&buff, as.fset, n); err != nil {
		return nil, fmt.Errorf("format example: %w", err)
	}
	src := gosrc.FormatExample(buff.Bytes())

	tokens, err := as.lexer.Lex(src)
	if err != nil {
		return nil, fmt.Errorf("highlight example: %w", err)
	}

	return &highlight.Code{
		Spans: []highlight.Span{
			&highlight.TokenSpan{Tokens: tokens},
		},
	}, nil
}
