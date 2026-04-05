package gosrc

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/scanner"
	"go/token"
	"go/types"

	"braces.dev/errtrace"
)

// TypesInfo provides type and scope information for identifiers.
type TypesInfo interface {
	// ObjectOf returns the object denoted by the given identifier,
	// or nil if the identifier is not in the Uses or Defs maps.
	ObjectOf(id *ast.Ident) types.Object

	// ScopeOf returns the lexical scope associated with the given node,
	// or nil if the node does not define a scope.
	ScopeOf(node ast.Node) *types.Scope
}

type typesInfoAdapter struct {
	info *types.Info
}

// AdaptTypesInfo adapts [types.Info] to [TypesInfo].
func AdaptTypesInfo(info *types.Info) TypesInfo {
	return typesInfoAdapter{info: info}
}

func (a typesInfoAdapter) ObjectOf(id *ast.Ident) types.Object {
	return a.info.ObjectOf(id)
}

func (a typesInfoAdapter) ScopeOf(node ast.Node) *types.Scope {
	return a.info.Scopes[node]
}

// DeclFormatter formats declarations from a single Go package.
//
// This may be re-used between declarations, but not across packages.
type DeclFormatter struct {
	fset     *token.FileSet
	files    []*ast.File
	topLevel map[string]struct{}
	debug    bool
	info     TypesInfo
}

// NewDeclFormatter builds a new DeclFormatter for the given package.
func NewDeclFormatter(fset *token.FileSet, files []*ast.File, topLevelDecls []string, info TypesInfo) *DeclFormatter {
	topLevel := make(map[string]struct{}, len(topLevelDecls))
	for _, name := range topLevelDecls {
		topLevel[name] = struct{}{}
	}

	return &DeclFormatter{
		fset:     fset,
		files:    files,
		topLevel: topLevel,
		info:     info,
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
		file:     f.fileForDecl(decl),
		topLevel: f.topLevel,
		info:     f.info,
	}
	lb.walk(decl)

	var buff bytes.Buffer
	if err := format.Node(&buff, f.fset, decl); err != nil {
		return nil, nil, errtrace.Wrap(fmt.Errorf("format decl: %w", err))
	}
	src = buff.Bytes()

	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var scan scanner.Scanner
	scan.Init(file, src, nil, scanner.ScanComments)

	remaining := lb.labels
	pos, tok, lit := scan.Scan()

	for ; tok != token.EOF; pos, tok, lit = scan.Scan() {
		var label Label
		if tok == token.IDENT {
			// There's an identifier but no label for it.
			// This is a bug. Fail silently.
			if len(remaining) == 0 {
				// TODO: Don't fail silently.
				// Log this and tell users to try debug mode.
				if !f.debug {
					break
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

func (f *DeclFormatter) fileForDecl(decl ast.Decl) *ast.File {
	for _, file := range f.files {
		if decl.Pos() >= file.Pos() && decl.Pos() <= file.End() {
			return file
		}
	}
	return nil
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
)

func (*DeclLabel) label()       {}
func (*EntityRefLabel) label()  {}
func (*PackageRefLabel) label() {}

// labeler traverses the AST for a declaration
// and for each identifier in the tree,
// records decorations for text that signify anchors and external links.
//
// They both rely on traversing visiting these identifiers
// in the same order as go/scanner -- so the order in which
// they appear in the text left to right.
type labeler struct {
	// labels are emitted in identifier order
	// and must stay aligned with the scanner's token stream.
	labels []Label

	// parents tracks the current declaration owner names
	// while we descend through the AST.
	// This stores the owner names we need for anchor labels
	// like "Type.Field", not the enclosing nodes themselves.
	// For example, when labeling the field "Field" inside type "Type",
	// parents holds ["Type"] and the field label contributes "Field".
	parents []string

	// stack tracks the current AST path.
	// It runs from the declaration root to the current node, inclusive,
	// with the current node at the end.
	stack []ast.Node

	file     *ast.File           // containing file for scope fallback
	topLevel map[string]struct{} // required
	info     TypesInfo           // required
}

func (lb *labeler) walk(root ast.Node) {
	ast.Walk(lb, root)
}

var _ ast.Visitor = (*labeler)(nil)

func (lb *labeler) Visit(n ast.Node) (next ast.Visitor) {
	if n == nil {
		// ast.Walk calls Visit(nil) after it finishes a node's children.
		// Use that post-order callback to keep the AST stack in sync with
		// the subtree we just left.
		if len(lb.stack) == 0 {
			return nil
		}

		lb.stack = lb.stack[:len(lb.stack)-1]
		return nil
	}

	lb.stack = append(lb.stack, n)
	defer func() {
		// If Visit returns nil, ast.Walk stops at this node
		// and does not issue Visit(nil) for it.
		// Pop the stack entry here so manually handled nodes keep the
		// explicit AST stack balanced.
		if next == nil {
			lb.stack = lb.stack[:len(lb.stack)-1]
		}
	}()

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
		// Package-qualified selectors are handled as a unit so that "pkg"
		// and "Name" stay paired as package/entity labels in scanner order.
		// Non-package selectors fall back to the normal traversal path.
		if !lb.packageEntityRef(n) {
			ast.Walk(lb, n.X)
			lb.ignore() // "Bar" of "foo.Bar"
		}

	case *ast.Ident:
		name := n.Name
		switch {
		case doc.IsPredeclared(name):
			lb.add(&EntityRefLabel{
				ImportPath: Builtin,
				Name:       name,
			})

		case ast.IsExported(name) && lb.isTopLevel(name):
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
	// Uses types.Info.ObjectOf to distinguish package references from
	// shadowed local variables. If x refers to a package import, ObjectOf
	// returns a *types.PkgName; if it's a local variable or other binding,
	// ObjectOf returns a different types.Object kind or nil.
	// If ObjectOf returns nil because type checking did not fully resolve
	// the selector expression, fall back to lexical scope lookup.
	x, _ := n.X.(*ast.Ident)
	if x == nil {
		return false
	}

	var pkgName *types.PkgName
	if obj := lb.info.ObjectOf(x); obj != nil {
		var ok bool
		pkgName, ok = obj.(*types.PkgName)
		if !ok {
			return false
		}
	} else {
		// Walk outward from the innermost enclosing scope.
		// This preserves normal shadowing rules for locals and type params
		// while still recovering file-scope imports when Uses is incomplete.
		for i := len(lb.stack) - 1; i >= 0; i-- {
			scope := lb.info.ScopeOf(lb.stack[i])
			if scope == nil {
				continue
			}

			_, obj := scope.LookupParent(x.Name, x.Pos())
			var ok bool
			pkgName, ok = obj.(*types.PkgName)
			if ok {
				break
			}
			if obj != nil {
				return false
			}
		}
		if pkgName == nil {
			// FormatDecl walks a declaration subtree, so the explicit AST stack
			// does not include the containing *ast.File. Imported package names
			// live in file scope, so fall back to the declaration's file scope
			// if no narrower scope on the declaration path resolved the name.
			if lb.file == nil {
				return false
			}
			scope := lb.info.ScopeOf(lb.file)
			if scope == nil {
				return false
			}
			_, obj := scope.LookupParent(x.Name, x.Pos())
			var ok bool
			pkgName, ok = obj.(*types.PkgName)
			if !ok {
				return false
			}
		}
	}

	importPath := pkgName.Imported().Path()
	lb.add(&PackageRefLabel{
		ImportPath: importPath,
	})
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
