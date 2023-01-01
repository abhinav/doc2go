package gosrc

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/scanner"
	"go/token"
	"strconv"
)

// DeclFormatter formats declarations from a single Go package.
//
// This may be re-used between declarations, but not across packages.
type DeclFormatter struct {
	fset     *token.FileSet
	topLevel map[string]struct{}
	debug    bool
}

// NewDeclFormatter builds a new DeclFormatter for the given package.
func NewDeclFormatter(fset *token.FileSet, topLevelDecls []string) *DeclFormatter {
	topLevel := make(map[string]struct{}, len(topLevelDecls))
	for _, name := range topLevelDecls {
		topLevel[name] = struct{}{}
	}

	return &DeclFormatter{
		fset:     fset,
		topLevel: topLevel,
	}
}

// Debug sets whether the formatter is in debug mode.
// In debug mode, the formatter may panic.
func (f *DeclFormatter) Debug(debug bool) {
	f.debug = debug
}

// FormatDecl formats a declaration back into source code,
// and reports regions inside it where anything of note happens.
func (f *DeclFormatter) FormatDecl(decl ast.Decl) (src []byte, regions []Region, err error) {
	lb := labeler{
		topLevel: f.topLevel,
	}
	ast.Walk(&lb, decl)

	var buff bytes.Buffer
	if err := format.Node(&buff, f.fset, decl); err != nil {
		return nil, nil, fmt.Errorf("format decl: %w", err)
	}
	src = buff.Bytes()

	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var scan scanner.Scanner
	scan.Init(file, src, nil, scanner.ScanComments)

	remaining := lb.labels
	pos, tok, lit := scan.Scan()

loop:
	for ; tok != token.EOF; pos, tok, lit = scan.Scan() {
		var label Label
		switch tok {
		case token.COMMENT:
			label = new(CommentLabel)

		case token.IDENT:
			// There's an identifier but no label for it.
			// This is a bug. Fail silently.
			if len(remaining) == 0 {
				// TODO: Don't fail silently.
				// Log this and tell users to try debug mode.
				if !f.debug {
					break loop
				}

				panic(fmt.Sprintf("Ran out of labels rendering:\n%s\nHave: %#v\nRemaining: %q", buff.String(), lb.labels, src[file.Offset(pos):]))
			}
			label, remaining = remaining[0], remaining[1:]
		}

		if label == nil {
			// Ignore this token.
			continue
		}
		regions = append(regions, Region{
			Label:  label,
			Offset: file.Offset(pos),
			Length: len(lit),
		})
	}

	return buff.Bytes(), regions, nil
}

// Region is a region of a declaration's source code
// that represents something special.
//
// Inside formatted source code src,
// a region r's label applies to:
//
//	src[r.Offset:r.Offset+r.Length]
type Region struct {
	// Label signifying what's special about this region.
	Label Label

	// Byte offset inside the formatted source code
	// where this region begins.
	Offset int

	// Length of this region.
	Length int
}

// Builtin is the value for [EntityRefLabel.ImportPath] if the entity
// referenced is a Go built-in.
const Builtin = "builtin"

type (
	// Label holds structured information
	// about a [Region].
	Label interface{ label() }

	// DeclLabel marks declaration sites
	// for struct fields, interface methods,
	// and vars and consts.
	DeclLabel struct {
		// Name of the parent inside which the child is declared.
		// Empty for vars and consts.
		Parent string

		// Name of the declared entity.
		Name string
	}

	// EntityRefLabel marks a region that references another entity.
	EntityRefLabel struct {
		// Import path of the package defining the referenced entity.
		//
		// This is empty for local references, and "builtin" for
		// built-ins.
		ImportPath string

		// Name of the entity referenced.
		Name string
	}

	// PackageRefLabel marks a region that references another Go package.
	PackageRefLabel struct {
		// Import path of the package.
		ImportPath string
	}

	// CommentLabel marks a region of a declaration that's a comment.
	CommentLabel struct{}
)

func (*DeclLabel) label()       {}
func (*EntityRefLabel) label()  {}
func (*PackageRefLabel) label() {}
func (*CommentLabel) label()    {}

// labeler traverses the AST for a declaration
// and for each identifier in the tree,
// records decorations for text that signify anchors and external links.
//
// They both rely on traversing visiting these identifiers
// in the same order as go/scanner -- so the order in which
// they appear in the text left to right.
type labeler struct {
	labels   []Label
	parents  []string
	topLevel map[string]struct{}
}

var _ ast.Visitor = (*labeler)(nil)

func (lb *labeler) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.TypeSpec:
		lb.ignore() // type name
		lb.pushParent(n.Name.Name)
		if n.TypeParams != nil {
			ast.Walk(lb, n.TypeParams)
		}
		ast.Walk(lb, n.Type)
		lb.popParent()

	case *ast.StructType, *ast.InterfaceType:
		var fields []*ast.Field
		// Double switch is a bit icky.
		switch n := n.(type) {
		case *ast.StructType:
			fields = n.Fields.List
		case *ast.InterfaceType:
			fields = n.Methods.List
		}

		parent := lb.parent()
		for _, f := range fields {
			for _, name := range f.Names {
				lb.add(&DeclLabel{
					Parent: parent,
					Name:   name.Name,
				})
			}
			ast.Walk(lb, f.Type)
		}

	case *ast.FuncDecl:
		if n.Recv != nil {
			ast.Walk(lb, n.Recv)
		}
		lb.ignore() // function/method name
		ast.Walk(lb, n.Type)

	case *ast.Field:
		// All field lists that we care to declare labels for
		// (struct fields and interface methods)
		// have already been handled.
		//
		// Only function parameters will make it here.
		for range n.Names {
			lb.ignore()
		}
		ast.Walk(lb, n.Type)

	case *ast.ValueSpec:
		for _, name := range n.Names {
			lb.add(&DeclLabel{Name: name.Name})
		}

		if n.Type != nil {
			ast.Walk(lb, n.Type)
		}

		for _, v := range n.Values {
			ast.Walk(lb, v)
		}

	case *ast.SelectorExpr:
		if !lb.packageEntityRef(n) {
			// If this wasn't a package entity reference,
			// fall back to traversing.
			ast.Walk(lb, n.X)
			lb.ignore() // "Bar" of "foo.Bar"
		}

	case *ast.Ident:
		name := n.Name
		switch {
		case n.Obj == nil && doc.IsPredeclared(name):
			lb.add(&EntityRefLabel{
				ImportPath: Builtin,
				Name:       name,
			})
		case n.Obj != nil && ast.IsExported(name) && lb.isTopLevel(name):
			// We need to filter to top-leve exported declarations
			// to avoid generating links for type parameters.
			lb.add(&EntityRefLabel{
				Name: name,
			})

		default:
			lb.ignore()
		}

		// TODO: long literal truncation
		// case *ast.BasicLit, *ast.CompositeLit:

	default:
		return lb
	}
	return nil
}

func (lb *labeler) packageEntityRef(n *ast.SelectorExpr) (ok bool) {
	x, _ := n.X.(*ast.Ident)
	if x == nil {
		return false
	}

	obj := x.Obj
	if obj == nil || obj.Kind != ast.Pkg {
		return false
	}

	spec, _ := obj.Decl.(*ast.ImportSpec)
	if spec == nil {
		return false
	}

	importPath, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return false // unreachable for all valid ASTs
	}

	lb.add(&PackageRefLabel{ImportPath: importPath})
	if importPath == "C" {
		lb.ignore()
	} else {
		lb.add(&EntityRefLabel{
			ImportPath: importPath,
			Name:       n.Sel.Name,
		})
	}

	return true
}

func (lb *labeler) parent() string {
	if n := len(lb.parents); n > 0 {
		return lb.parents[n-1]
	}
	return ""
}

func (lb *labeler) pushParent(name string) {
	lb.parents = append(lb.parents, name)
}

func (lb *labeler) popParent() {
	lb.parents = lb.parents[:len(lb.parents)-1]
}

func (lb *labeler) ignore() { lb.add(nil) }

func (lb *labeler) add(l Label) {
	lb.labels = append(lb.labels, l)
}

func (lb *labeler) isTopLevel(name string) bool {
	_, ok := lb.topLevel[name]
	return ok
}
