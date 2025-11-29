package gosrc

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeclFormatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string

		// One or more declarations.
		// Only the last of these will be formatted;
		// the others are auxilliary.
		// Can be a single string or a slice of lines.
		give any // string or []string

		imports map[string]string // import path => package name

		// List of top-level declarations.
		topLevel []string

		// Formatted form of the last declaration in 'give'.
		want any // string or []string

		// Special regions of 'want'.
		regions []Region
	}{
		{
			desc:     "struct",
			give:     "type Foo struct{ Bar string }",
			topLevel: []string{"Foo"},
			want:     "type Foo struct{ «Bar» «string» }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Foo", Name: "Bar"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
			},
		},
		{
			desc: "interface",
			give: []string{
				"type Bar struct{}",
				"type Foo interface{ Bar(x Bar) }",
			},
			topLevel: []string{"Foo", "Bar"},
			want:     "type Foo interface{ «Bar»(x «Bar») }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Foo", Name: "Bar"}},
				{Label: &EntityRefLabel{Name: "Bar"}},
			},
		},
		{
			desc:     "function",
			give:     "func Foo(s string) int",
			topLevel: []string{"Foo"},
			want:     "func Foo(s «string») «int»",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
			},
		},
		{
			desc: "method",
			give: []string{
				"type Foo struct{}",
				"type Baz struct{}",
				"func (*Foo) Bar() Baz",
			},
			topLevel: []string{"Foo", "Baz"},
			want:     "func (*«Foo») Bar() «Baz»",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Foo"}},
				{Label: &EntityRefLabel{Name: "Baz"}},
			},
		},
		{
			desc:     "value",
			give:     `var Foo = "foo"`,
			topLevel: []string{"Foo"},
			want:     `var «Foo» = "foo"`,
			regions: []Region{
				{Label: &DeclLabel{Name: "Foo"}},
			},
		},
		{
			desc: "value with type",
			give: []string{
				"type Bar int",
				"const Foo Bar = iota",
			},
			topLevel: []string{"Foo", "Bar"},
			want:     `const «Foo» «Bar» = «iota»`,
			regions: []Region{
				{Label: &DeclLabel{Name: "Foo"}},
				{Label: &EntityRefLabel{Name: "Bar"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "iota"}},
			},
		},
		{
			desc: "struct with comment",
			give: []string{
				"type Foo struct {",
				"	// Bar does things.",
				"	Bar string",
				"}",
			},
			topLevel: []string{"Foo"},
			want: []string{
				"type Foo struct {",
				"	// Bar does things.",
				"	«Bar» «string»",
				"}",
			},
			regions: []Region{
				{Label: &DeclLabel{Parent: "Foo", Name: "Bar"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
			},
		},
		{
			desc: "imported",
			give: "type Foo bar.Baz",
			imports: map[string]string{
				"example.com/bar-go": "bar",
			},
			topLevel: []string{"Foo"},
			want:     "type Foo «bar».«Baz»",
			regions: []Region{
				{Label: &PackageRefLabel{ImportPath: "example.com/bar-go"}},
				{Label: &EntityRefLabel{ImportPath: "example.com/bar-go", Name: "Baz"}},
			},
		},
		{
			desc: "type params",
			give: []string{
				"type Bar interface{ X() }",
				"type Foo[T Bar] struct{ Field T }",
			},
			topLevel: []string{"Foo", "Bar"},
			want:     "type Foo[T «Bar»] struct{ «Field» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Bar"}},
				{Label: &DeclLabel{Parent: "Foo", Name: "Field"}},
			},
		},
		{
			desc: "non-package ref",
			give: []string{
				`var foo = struct{ Name string }{Name: "foo"}`,
				"var Foo = foo.Name",
			},
			topLevel: []string{"Foo"},
			want:     "var «Foo» = foo.Name",
			regions: []Region{
				{Label: &DeclLabel{Name: "Foo"}},
			},
		},
		{
			desc:     "parameter shadowing builtin in type position",
			give:     "func Foo(string string) int",
			topLevel: []string{"Foo"},
			want:     "func Foo(string «string») «int»",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
			},
		},
		{
			desc: "parameter shadowing exported type in type position",
			give: []string{
				"type Bar struct{}",
				"func Foo(Bar Bar) int",
			},
			topLevel: []string{"Foo", "Bar"},
			want:     "func Foo(Bar «Bar») «int»",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Bar"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
			},
		},
		{
			desc: "local variable shadowing imported package",
			give: `var foo string
type Foo bar.Baz`,
			imports: map[string]string{
				"example.com/bar": "bar",
			},
			topLevel: []string{"Foo", "foo"},
			want:     "type Foo «bar».«Baz»",
			regions: []Region{
				{Label: &PackageRefLabel{ImportPath: "example.com/bar"}},
				{Label: &EntityRefLabel{ImportPath: "example.com/bar", Name: "Baz"}},
			},
		},
		{
			desc: "selector on non-package local variable",
			give: `var s struct { Field string }
type Foo s.Field`,
			topLevel: []string{"Foo", "s"},
			want:     "type Foo s.Field",
			// When s is a local variable (not a package),
			// packageEntityRef returns false and we fall back to ast.Walk.
			// This causes the entire selector expression to be ignored.
			regions: nil,
		},
		{
			desc:     "type alias to primitive builtin",
			give:     "type MyInt = int",
			topLevel: []string{"MyInt"},
			want:     "type MyInt = «int»",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
			},
		},
		{
			desc:     "type alias to builtin complex type",
			give:     "type StringSlice = []string",
			topLevel: []string{"StringSlice"},
			want:     "type StringSlice = []«string»",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
			},
		},
		{
			desc: "type alias to local custom type",
			give: []string{
				"type Base struct { X int }",
				"type Alias = Base",
			},
			topLevel: []string{"Base", "Alias"},
			want:     "type Alias = «Base»",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Base"}},
			},
		},
		{
			desc: "type alias to imported type",
			give: "type MyReader = io.Reader",
			imports: map[string]string{
				"io": "io",
			},
			topLevel: []string{"MyReader"},
			want:     "type MyReader = «io».«Reader»",
			regions: []Region{
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Reader"}},
			},
		},
		{
			desc:     "generic type with any constraint",
			give:     "type Container[T any] struct { Value T }",
			topLevel: []string{"Container"},
			want:     "type Container[T «any»] struct{ «Value» T }",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &DeclLabel{Parent: "Container", Name: "Value"}},
			},
		},
		{
			desc:     "generic type with explicit any",
			give:     "type Box[T any] struct { Item T }",
			topLevel: []string{"Box"},
			want:     "type Box[T «any»] struct{ «Item» T }",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &DeclLabel{Parent: "Box", Name: "Item"}},
			},
		},
		{
			desc: "generic with local type constraint",
			give: []string{
				"type Writer interface { Write() }",
				"type Logger[T Writer] struct { W T }",
			},
			topLevel: []string{"Writer", "Logger"},
			want:     "type Logger[T «Writer»] struct{ «W» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Writer"}},
				{Label: &DeclLabel{Parent: "Logger", Name: "W"}},
			},
		},
		{
			desc: "generic with imported type constraint",
			give: "type Process[T io.Reader] struct { R T }",
			imports: map[string]string{
				"io": "io",
			},
			topLevel: []string{"Process"},
			want:     "type Process[T «io».«Reader»] struct{ «R» T }",
			regions: []Region{
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Reader"}},
				{Label: &DeclLabel{Parent: "Process", Name: "R"}},
			},
		},
		{
			desc: "generic with union of builtins",
			give: []string{
				"type Number interface { int | int64 | float64 }",
				"type Numeric[T Number] struct { Val T }",
			},
			topLevel: []string{"Number", "Numeric"},
			want:     "type Numeric[T «Number»] struct{ «Val» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Number"}},
				{Label: &DeclLabel{Parent: "Numeric", Name: "Val"}},
			},
		},
		{
			desc:     "generic with comparable constraint",
			give:     "type Set[T comparable] struct { items []T }",
			topLevel: []string{"Set"},
			want:     "type Set[T «comparable»] struct{ «items» []T }",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "comparable"}},
				{Label: &DeclLabel{Parent: "Set", Name: "items"}},
			},
		},
		{
			desc:     "generic with multiple params, different constraints",
			give:     "type Pair[K comparable, V any] struct { K K; V V }",
			topLevel: []string{"Pair"},
			want: []string{
				"type Pair[K «comparable», V «any»] struct {",
				"	«K» K",
				"	«V» V",
				"}",
			},
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "comparable"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &DeclLabel{Parent: "Pair", Name: "K"}},
				{Label: &DeclLabel{Parent: "Pair", Name: "V"}},
			},
		},
		{
			desc: "generic with three params including imported constraint",
			give: "type Triple[A io.Reader, B any, C comparable] struct { A A; B B; C C }",
			imports: map[string]string{
				"io": "io",
			},
			topLevel: []string{"Triple"},
			want: []string{
				"type Triple[A «io».«Reader», B «any», C «comparable»] struct {",
				"	«A» A",
				"	«B» B",
				"	«C» C",
				"}",
			},
			regions: []Region{
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Reader"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "comparable"}},
				{Label: &DeclLabel{Parent: "Triple", Name: "A"}},
				{Label: &DeclLabel{Parent: "Triple", Name: "B"}},
				{Label: &DeclLabel{Parent: "Triple", Name: "C"}},
			},
		},
		{
			desc: "generic with nested generic constraint",
			give: []string{
				"type Iterator[T any] interface { Next() T }",
				"type Stream[T Iterator[int]] struct { I T }",
			},
			topLevel: []string{"Iterator", "Stream"},
			want:     "type Stream[T «Iterator»[«int»]] struct{ «I» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Iterator"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
				{Label: &DeclLabel{Parent: "Stream", Name: "I"}},
			},
		},
		{
			desc: "generic with constraint requiring exported method",
			give: []string{
				"type Stringer interface { String() string }",
				"type Wrapped[T Stringer] struct { Inner T }",
			},
			topLevel: []string{"Stringer", "Wrapped"},
			want:     "type Wrapped[T «Stringer»] struct{ «Inner» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Stringer"}},
				{Label: &DeclLabel{Parent: "Wrapped", Name: "Inner"}},
			},
		},
		{
			desc: "constraint references another generic type",
			give: []string{
				"type Converter[T any] interface { Convert() T }",
				"type Pipeline[T Converter[string]] struct { C T }",
			},
			topLevel: []string{"Converter", "Pipeline"},
			want:     "type Pipeline[T «Converter»[«string»]] struct{ «C» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Converter"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
				{Label: &DeclLabel{Parent: "Pipeline", Name: "C"}},
			},
		},
		{
			desc:     "generic alias to slice of type param",
			give:     "type List[T any] = []T",
			topLevel: []string{"List"},
			want:     "type List[T «any»] = []T",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
			},
		},
		{
			desc:     "generic alias to map",
			give:     "type Dict[K comparable, V any] = map[K]V",
			topLevel: []string{"Dict"},
			want:     "type Dict[K «comparable», V «any»] = map[K]V",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "comparable"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
			},
		},
		{
			desc:     "generic alias to channel",
			give:     "type Chan[T any] = chan T",
			topLevel: []string{"Chan"},
			want:     "type Chan[T «any»] = chan T",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
			},
		},
		{
			desc: "generic alias to generic custom type",
			give: []string{
				"type Container[T any] struct { V T }",
				"type Wrapped[T any] = Container[T]",
			},
			topLevel: []string{"Container", "Wrapped"},
			want:     "type Wrapped[T «any»] = «Container»[T]",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &EntityRefLabel{Name: "Container"}},
			},
		},
		{
			desc: "generic alias with constrained type param",
			give: []string{
				"type Numbers interface { int | float64 }",
				"type NumSlice[T Numbers] = []T",
			},
			topLevel: []string{"Numbers", "NumSlice"},
			want:     "type NumSlice[T «Numbers»] = []T",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Numbers"}},
			},
		},
		{
			desc:     "generic alias to nested generic",
			give:     "type Matrix[T any] = [][]T",
			topLevel: []string{"Matrix"},
			want:     "type Matrix[T «any»] = [][]T",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
			},
		},
		{
			desc:     "self-referencing struct",
			give:     "type Node struct { Value int; Next *Node }",
			topLevel: []string{"Node"},
			want: []string{
				"type Node struct {",
				"	«Value» «int»",
				"	«Next»  *«Node»",
				"}",
			},
			regions: []Region{
				{Label: &DeclLabel{Parent: "Node", Name: "Value"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
				{Label: &DeclLabel{Parent: "Node", Name: "Next"}},
				{Label: &EntityRefLabel{Name: "Node"}},
			},
		},
		{
			desc:     "self-referencing generic struct",
			give:     "type LinkedList[T any] struct { Value T; Next *LinkedList[T] }",
			topLevel: []string{"LinkedList"},
			want: []string{
				"type LinkedList[T «any»] struct {",
				"	«Value» T",
				"	«Next»  *«LinkedList»[T]",
				"}",
			},
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &DeclLabel{Parent: "LinkedList", Name: "Value"}},
				{Label: &DeclLabel{Parent: "LinkedList", Name: "Next"}},
				{Label: &EntityRefLabel{Name: "LinkedList"}},
			},
		},
		{
			desc: "mutually recursive types",
			give: []string{
				"type Foo struct { B *Bar }",
				"type Bar struct { F *Foo }",
			},
			topLevel: []string{"Foo", "Bar"},
			want:     "type Bar struct{ «F» *«Foo» }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Bar", Name: "F"}},
				{Label: &EntityRefLabel{Name: "Foo"}},
			},
		},
		{
			desc: "mutually recursive generics",
			give: []string{
				"type Foo[T any] struct { B *Bar[T] }",
				"type Bar[T any] struct { F *Foo[T] }",
			},
			topLevel: []string{"Foo", "Bar"},
			want:     "type Bar[T «any»] struct{ «F» *«Foo»[T] }",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &DeclLabel{Parent: "Bar", Name: "F"}},
				{Label: &EntityRefLabel{Name: "Foo"}},
			},
		},
		{
			desc: "recursive struct with imported type",
			give: "type Node struct { Value io.Reader; Next *Node }",
			imports: map[string]string{
				"io": "io",
			},
			topLevel: []string{"Node"},
			want: []string{
				"type Node struct {",
				"	«Value» «io».«Reader»",
				"	«Next»  *«Node»",
				"}",
			},
			regions: []Region{
				{Label: &DeclLabel{Parent: "Node", Name: "Value"}},
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Reader"}},
				{Label: &DeclLabel{Parent: "Node", Name: "Next"}},
				{Label: &EntityRefLabel{Name: "Node"}},
			},
		},
		{
			desc: "recursive generic with constraint",
			give: []string{
				"type Comparable interface { Compare(Comparable) int }",
				"type Tree[T Comparable] struct { Left *Tree[T]; Right *Tree[T] }",
			},
			topLevel: []string{"Comparable", "Tree"},
			want: []string{
				"type Tree[T «Comparable»] struct {",
				"	«Left»  *«Tree»[T]",
				"	«Right» *«Tree»[T]",
				"}",
			},
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Comparable"}},
				{Label: &DeclLabel{Parent: "Tree", Name: "Left"}},
				{Label: &EntityRefLabel{Name: "Tree"}},
				{Label: &DeclLabel{Parent: "Tree", Name: "Right"}},
				{Label: &EntityRefLabel{Name: "Tree"}},
			},
		},
		{
			desc: "recursive with embedded type reference",
			give: []string{
				"type Base struct { X int }",
				"type Extended struct { Base; Next *Extended }",
			},
			topLevel: []string{"Base", "Extended"},
			want: []string{
				"type Extended struct {",
				"	«Base»",
				"	«Next» *«Extended»",
				"}",
			},
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Base"}},
				{Label: &DeclLabel{Parent: "Extended", Name: "Next"}},
				{Label: &EntityRefLabel{Name: "Extended"}},
			},
		},
		{
			desc:     "deep recursive generics",
			give:     "type Wrapper[T any] struct { Inner *Wrapper[*T] }",
			topLevel: []string{"Wrapper"},
			want:     "type Wrapper[T «any»] struct{ «Inner» *«Wrapper»[*T] }",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &DeclLabel{Parent: "Wrapper", Name: "Inner"}},
				{Label: &EntityRefLabel{Name: "Wrapper"}},
			},
		},
		{
			desc:     "function with generic type parameter",
			give:     "func Process[T any](val T) T",
			topLevel: []string{"Process"},
			want:     "func Process[T «any»](val T) T",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
			},
		},
		{
			desc: "function with constrained generic",
			give: []string{
				"type Reader interface { Read() }",
				"func Handle[T Reader](r T) error",
			},
			topLevel: []string{"Handle", "Reader"},
			want:     "func Handle[T «Reader»](r T) «error»",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Reader"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "error"}},
			},
		},
		{
			desc: "function with imported generic constraint",
			give: "func Work[T io.Reader](r T) ([]byte, error)",
			imports: map[string]string{
				"io": "io",
			},
			topLevel: []string{"Work"},
			want:     "func Work[T «io».«Reader»](r T) ([]«byte», «error»)",
			regions: []Region{
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Reader"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "byte"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "error"}},
			},
		},
		{
			desc: "method on generic type",
			give: []string{
				"type Container[T any] struct { V T }",
				"func (*Container[T]) Get() T",
			},
			topLevel: []string{"Container", "Get"},
			want:     "func (*«Container»[T]) Get() T",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Container"}},
			},
		},
		{
			desc: "method on generic type with constraint",
			give: []string{
				"type Comparable interface { Compare(Comparable) int }",
				"type Holder[T Comparable] struct { V T }",
				"func (*Holder[T]) IsEqual(other T) bool",
			},
			topLevel: []string{"Comparable", "Holder"},
			want:     "func (*«Holder»[T]) IsEqual(other T) «bool»",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Holder"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "bool"}},
			},
		},
		{
			desc:     "function with multiple constrained generics",
			give:     "func Convert[S any, D comparable](s S) D",
			topLevel: []string{"Convert"},
			want:     "func Convert[S «any», D «comparable»](s S) D",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "comparable"}},
			},
		},
		{
			desc: "function returning generic type",
			give: []string{
				"type Box[T any] struct { V T }",
				"func Wrap[T any](v T) Box[T]",
			},
			topLevel: []string{"Box", "Wrap"},
			want:     "func Wrap[T «any»](v T) «Box»[T]",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &EntityRefLabel{Name: "Box"}},
			},
		},
		{
			desc: "function with generic and imported types",
			give: "func Read[T any](r io.Reader) (T, error)",
			imports: map[string]string{
				"io": "io",
			},
			topLevel: []string{"Read"},
			want:     "func Read[T «any»](r «io».«Reader») (T, «error»)",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Reader"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "error"}},
			},
		},
		{
			desc: "struct with generic field",
			give: []string{
				"type Container[T any] struct { V T }",
				"type Wrapper struct { C Container[string] }",
			},
			topLevel: []string{"Container", "Wrapper"},
			want:     "type Wrapper struct{ «C» «Container»[«string»] }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Wrapper", Name: "C"}},
				{Label: &EntityRefLabel{Name: "Container"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
			},
		},
		{
			desc: "struct with imported generic field",
			give: "type Config struct { M map[string]bytes.Buffer }",
			imports: map[string]string{
				"bytes": "bytes",
			},
			topLevel: []string{"Config"},
			want:     "type Config struct{ «M» map[«string»]«bytes».«Buffer» }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Config", Name: "M"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
				{Label: &PackageRefLabel{ImportPath: "bytes"}},
				{Label: &EntityRefLabel{ImportPath: "bytes", Name: "Buffer"}},
			},
		},
		{
			desc:     "struct with function type field",
			give:     "type Callback struct { F func(int, string) bool }",
			topLevel: []string{"Callback"},
			want:     "type Callback struct{ «F» func(«int», «string») «bool» }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Callback", Name: "F"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "bool"}},
			},
		},
		{
			desc: "struct with embedded interface",
			give: []string{
				"type Reader interface { Read() }",
				"type Handler struct { Reader }",
			},
			topLevel: []string{"Reader", "Handler"},
			want:     "type Handler struct{ «Reader» }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Reader"}},
			},
		},
		{
			desc:     "struct with channel field",
			give:     "type Pipeline struct { Ch chan int; Done chan bool }",
			topLevel: []string{"Pipeline"},
			want: []string{
				"type Pipeline struct {",
				"	«Ch»   chan «int»",
				"	«Done» chan «bool»",
				"}",
			},
			regions: []Region{
				{Label: &DeclLabel{Parent: "Pipeline", Name: "Ch"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
				{Label: &DeclLabel{Parent: "Pipeline", Name: "Done"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "bool"}},
			},
		},
		{
			desc: "struct with slice of imported type",
			give: "type Batch struct { Items []json.RawMessage }",
			imports: map[string]string{
				"encoding/json": "json",
			},
			topLevel: []string{"Batch"},
			want:     "type Batch struct{ «Items» []«json».«RawMessage» }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Batch", Name: "Items"}},
				{Label: &PackageRefLabel{ImportPath: "encoding/json"}},
				{Label: &EntityRefLabel{ImportPath: "encoding/json", Name: "RawMessage"}},
			},
		},
		{
			desc: "struct field as pointer to generic",
			give: []string{
				"type Node[T any] struct { V T }",
				"type Tree struct { Root *Node[int] }",
			},
			topLevel: []string{"Node", "Tree"},
			want:     "type Tree struct{ «Root» *«Node»[«int»] }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Tree", Name: "Root"}},
				{Label: &EntityRefLabel{Name: "Node"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
			},
		},
		{
			desc: "struct with multiple complex field types",
			give: "type Handler struct { R io.Reader; W io.Writer; Err error; Timeout time.Duration }",
			imports: map[string]string{
				"io":   "io",
				"time": "time",
			},
			topLevel: []string{"Handler"},
			want: []string{
				"type Handler struct {",
				"	«R»       «io».«Reader»",
				"	«W»       «io».«Writer»",
				"	«Err»     «error»",
				"	«Timeout» «time».«Duration»",
				"}",
			},
			regions: []Region{
				{Label: &DeclLabel{Parent: "Handler", Name: "R"}},
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Reader"}},
				{Label: &DeclLabel{Parent: "Handler", Name: "W"}},
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Writer"}},
				{Label: &DeclLabel{Parent: "Handler", Name: "Err"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "error"}},
				{Label: &DeclLabel{Parent: "Handler", Name: "Timeout"}},
				{Label: &PackageRefLabel{ImportPath: "time"}},
				{Label: &EntityRefLabel{ImportPath: "time", Name: "Duration"}},
			},
		},
		{
			desc: "interface embedding generic interface",
			give: []string{
				"type Container[T any] interface { Get() T }",
				"type Consumer interface { Container[int] }",
			},
			topLevel: []string{"Container", "Consumer"},
			want:     "type Consumer interface{ «Container»[«int»] }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Container"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
			},
		},
		{
			desc: "interface method with imported return type",
			give: "type Driver interface { Read() (io.Reader, error) }",
			imports: map[string]string{
				"io": "io",
			},
			topLevel: []string{"Driver"},
			want:     "type Driver interface{ «Read»() («io».«Reader», «error») }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Driver", Name: "Read"}},
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Reader"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "error"}},
			},
		},
		{
			desc: "interface embedding imported interface",
			give: "type Extended interface { io.Reader; io.Writer }",
			imports: map[string]string{
				"io": "io",
			},
			topLevel: []string{"Extended"},
			want: []string{
				"type Extended interface {",
				"	«io».«Reader»",
				"	«io».«Writer»",
				"}",
			},
			regions: []Region{
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Reader"}},
				{Label: &PackageRefLabel{ImportPath: "io"}},
				{Label: &EntityRefLabel{ImportPath: "io", Name: "Writer"}},
			},
		},
		{
			desc: "union constraint with imported types",
			give: []string{
				"type Value interface { json.Number | json.RawMessage }",
				"type JSON[T Value] struct { V T }",
			},
			imports: map[string]string{
				"encoding/json": "json",
			},
			topLevel: []string{"Value", "JSON"},
			want:     "type JSON[T «Value»] struct{ «V» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Value"}},
				{Label: &DeclLabel{Parent: "JSON", Name: "V"}},
			},
		},
		{
			desc: "constraint interface with generic method",
			give: []string{
				"type Converter interface { Convert(any) }",
				"type Adapter[T Converter] struct { C T }",
			},
			topLevel: []string{"Converter", "Adapter"},
			want:     "type Adapter[T «Converter»] struct{ «C» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Converter"}},
				{Label: &DeclLabel{Parent: "Adapter", Name: "C"}},
			},
		},
		{
			desc: "multiple params with union constraints",
			give: []string{
				"type A interface { int | string }",
				"type B interface { float64 | bool }",
				"type Dual[X A, Y B] struct { A X; B Y }",
			},
			topLevel: []string{"A", "B", "Dual"},
			want: []string{
				"type Dual[X «A», Y «B»] struct {",
				"	«A» X",
				"	«B» Y",
				"}",
			},
			regions: []Region{
				{Label: &EntityRefLabel{Name: "A"}},
				{Label: &EntityRefLabel{Name: "B"}},
				{Label: &DeclLabel{Parent: "Dual", Name: "A"}},
				{Label: &DeclLabel{Parent: "Dual", Name: "B"}},
			},
		},
		{
			desc: "nested constraint referencing imported type",
			give: []string{
				"type Sized interface { Size() int }",
				"type Collection interface { json.Marshaler; Sized }",
				"type Holder[T Collection] struct { V T }",
			},
			imports: map[string]string{
				"encoding/json": "json",
			},
			topLevel: []string{"Sized", "Collection", "Holder"},
			want:     "type Holder[T «Collection»] struct{ «V» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Collection"}},
				{Label: &DeclLabel{Parent: "Holder", Name: "V"}},
			},
		},
		{
			desc: "constraint with multiple required methods",
			give: []string{
				"type ReadWriter interface { Read() ([]byte, error); Write([]byte) error }",
				"type Stream[T ReadWriter] struct { Handler T }",
			},
			topLevel: []string{"ReadWriter", "Stream"},
			want:     "type Stream[T «ReadWriter»] struct{ «Handler» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "ReadWriter"}},
				{Label: &DeclLabel{Parent: "Stream", Name: "Handler"}},
			},
		},
		{
			desc: "constraint referencing local constraint",
			give: []string{
				"type Base interface { String() string }",
				"type Extended interface { Base; Len() int }",
				"type Stack[T Extended] struct { Items []T }",
			},
			topLevel: []string{"Base", "Extended", "Stack"},
			want:     "type Stack[T «Extended»] struct{ «Items» []T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Extended"}},
				{Label: &DeclLabel{Parent: "Stack", Name: "Items"}},
			},
		},
		{
			desc: "generic alias to map of generic to generic",
			give: []string{
				"type Container[T any] struct { V T }",
				"type Index[K comparable, V any] = map[K]Container[V]",
			},
			topLevel: []string{"Container", "Index"},
			want:     "type Index[K «comparable», V «any»] = map[K]«Container»[V]",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "comparable"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &EntityRefLabel{Name: "Container"}},
			},
		},
		{
			desc:     "type alias to function type",
			give:     "type Handler = func(string) (int, error)",
			topLevel: []string{"Handler"},
			want:     "type Handler = func(«string») («int», «error»)",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "error"}},
			},
		},
		{
			desc: "generic struct with pointer field and constraint",
			give: []string{
				"type Comparable interface { Compare(Comparable) int }",
				"type Node[T Comparable] struct { Value T; Next *Node[T] }",
			},
			topLevel: []string{"Comparable", "Node"},
			want: []string{
				"type Node[T «Comparable»] struct {",
				"	«Value» T",
				"	«Next»  *«Node»[T]",
				"}",
			},
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Comparable"}},
				{Label: &DeclLabel{Parent: "Node", Name: "Value"}},
				{Label: &DeclLabel{Parent: "Node", Name: "Next"}},
				{Label: &EntityRefLabel{Name: "Node"}},
			},
		},
		{
			desc: "complex struct with generics, imports, and recursion",
			give: []string{
				"type Node[T any] struct {",
				"  Value T",
				"  Children []Node[T]",
				"  Meta json.RawMessage",
				"}",
			},
			imports: map[string]string{
				"encoding/json": "json",
			},
			topLevel: []string{"Node"},
			want: []string{
				"type Node[T «any»] struct {",
				"	«Value»    T",
				"	«Children» []«Node»[T]",
				"	«Meta»     «json».«RawMessage»",
				"}",
			},
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &DeclLabel{Parent: "Node", Name: "Value"}},
				{Label: &DeclLabel{Parent: "Node", Name: "Children"}},
				{Label: &EntityRefLabel{Name: "Node"}},
				{Label: &DeclLabel{Parent: "Node", Name: "Meta"}},
				{Label: &PackageRefLabel{ImportPath: "encoding/json"}},
				{Label: &EntityRefLabel{ImportPath: "encoding/json", Name: "RawMessage"}},
			},
		},
		{
			desc: "generic type using generic alias in constraint",
			give: []string{
				"type Container[T any] struct { V T }",
				"type AliasContainer[T any] = Container[T]",
				"type Wrapper[T AliasContainer[string]] struct { C T }",
			},
			topLevel: []string{"Container", "AliasContainer", "Wrapper"},
			want:     "type Wrapper[T «AliasContainer»[«string»]] struct{ «C» T }",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "AliasContainer"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
				{Label: &DeclLabel{Parent: "Wrapper", Name: "C"}},
			},
		},
		{
			desc: "interface embedding generic and adding methods",
			give: []string{
				"type Container[T any] interface { Get() T }",
				"type Enhanced interface { Container[int]; Len() int }",
			},
			topLevel: []string{"Container", "Enhanced"},
			want: []string{
				"type Enhanced interface {",
				"	«Container»[«int»]",
				"	«Len»() «int»",
				"}",
			},
			regions: []Region{
				{Label: &EntityRefLabel{Name: "Container"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
				{Label: &DeclLabel{Parent: "Enhanced", Name: "Len"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
			},
		},
		{
			desc: "reference to unexported type",
			give: []string{
				"type privateType int",
				"type Public struct { Private privateType }",
			},
			topLevel: []string{"Public", "privateType"},
			want:     "type Public struct{ «Private» privateType }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Public", Name: "Private"}},
			},
		},
		{
			desc: "generic with unexported constraint",
			give: []string{
				"type privateConstraint interface { Foo() }",
				"type Public[T privateConstraint] struct { V T }",
			},
			topLevel: []string{"Public", "privateConstraint"},
			want:     "type Public[T privateConstraint] struct{ «V» T }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Public", Name: "V"}},
			},
		},
		{
			desc:     "type param shadowing builtin",
			give:     "type Generic[string any] struct { F string }",
			topLevel: []string{"Generic"},
			want:     "type Generic[string «any»] struct{ «F» «string» }",
			regions: []Region{
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "any"}},
				{Label: &DeclLabel{Parent: "Generic", Name: "F"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
			},
		},
		{
			desc: "disambiguating imported packages",
			give: []string{
				"type A struct { F1 json.Number }",
				"type B struct { F2 bytes.Buffer }",
			},
			imports: map[string]string{
				"encoding/json": "json",
				"bytes":         "bytes",
			},
			topLevel: []string{"A", "B"},
			want:     "type B struct{ «F2» «bytes».«Buffer» }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "B", Name: "F2"}},
				{Label: &PackageRefLabel{ImportPath: "bytes"}},
				{Label: &EntityRefLabel{ImportPath: "bytes", Name: "Buffer"}},
			},
		},
		{
			desc:     "empty interface",
			give:     "type Any interface{}",
			topLevel: []string{"Any"},
			want:     "type Any interface{}",
			regions:  nil,
		},
		{
			desc:     "deep pointer indirection",
			give:     "type Deep struct { F *****int }",
			topLevel: []string{"Deep"},
			want:     "type Deep struct{ «F» *****«int» }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Deep", Name: "F"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "int"}},
			},
		},
		{
			desc: "generic with complex nested instantiation",
			give: []string{
				"type Outer[T any] struct { V T }",
				"type Middle[T any] struct { O Outer[[]T] }",
				"type Inner struct { M Middle[string] }",
			},
			topLevel: []string{"Outer", "Middle", "Inner"},
			want:     "type Inner struct{ «M» «Middle»[«string»] }",
			regions: []Region{
				{Label: &DeclLabel{Parent: "Inner", Name: "M"}},
				{Label: &EntityRefLabel{Name: "Middle"}},
				{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var giveStr string
			switch g := tt.give.(type) {
			case string:
				giveStr = g
			case []string:
				giveStr = strings.Join(g, "\n")
			default:
				t.Fatalf("invalid give type: %T", tt.give)
			}

			var wantStr string
			switch w := tt.want.(type) {
			case string:
				wantStr = w
			case []string:
				wantStr = strings.Join(w, "\n")
			default:
				t.Fatalf("invalid want type: %T", tt.want)
			}

			var input strings.Builder
			fmt.Fprintln(&input, "package foo")
			for path := range tt.imports {
				fmt.Fprintf(&input, "import %q\n", path)
			}
			fmt.Fprintln(&input, giveStr)

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "a.go", input.String(), parser.ParseComments)
			require.NoError(t, err)
			require.NotEmpty(t, file.Decls)

			pkgImports := make([]ImportedPackage, 0, len(tt.imports))
			for path, name := range tt.imports {
				pkgImports = append(pkgImports, ImportedPackage{
					Name:       name,
					ImportPath: path,
				})
			}
			slices.SortFunc(pkgImports, func(a, b ImportedPackage) int {
				return strings.Compare(a.ImportPath, b.ImportPath)
			})

			info := types.Info{
				Uses: make(map[*ast.Ident]types.Object),
				Defs: make(map[*ast.Ident]types.Object),
			}
			_, _ = (&types.Config{
				IgnoreFuncBodies:         true,
				FakeImportC:              true,
				Importer:                 newPackageImporter(pkgImports),
				Error:                    func(error) {},
				DisableUnusedImportCheck: true,
			}).Check("example.com/mypkg", fset, []*ast.File{file}, &info)

			df := NewDeclFormatter(fset, tt.topLevel, &info)
			df.Debug(true)
			src, gotRegions, err := df.FormatDecl(file.Decls[len(file.Decls)-1])
			require.NoError(t, err)

			defer func() {
				if t.Failed() {
					// On failure, print debug info.
					for i, r := range gotRegions {
						t.Logf("region[%d] = %+v (offset=%d, length=%d) = %q",
							i, r.Label, r.Offset, r.Length,
							string(src[r.Offset:r.Offset+r.Length]))
					}
				}
			}()

			// Parse want string to extract markers and fill in Offset/Length
			expectedWant, expectedRegions, err := parseRegionSpans(wantStr, tt.regions)
			require.NoError(t, err)

			assert.Equal(t, expectedWant, string(src))
			assert.Equal(t, expectedRegions, gotRegions)
		})
	}
}

// parseRegionSpans is a helper function that processes strings
// with special markers and populates Region offsets and lengths.
//
// The string uses « and » Unicode characters to mark the positions
// of labels within the formatted output. For example:
//
//	want: "type Foo struct{ «Bar» «string» }"
//	regions: []Region{
//		{Label: &DeclLabel{Parent: "Foo", Name: "Bar"}},
//		{Label: &EntityRefLabel{ImportPath: Builtin, Name: "string"}},
//	}
//
// parseRegionSpans extracts the markers from want in order, removes them,
// and fills in the Offset and Length fields of the corresponding regions.
//
// The regions slice must be in the same order as the markers appear in want.
// If the number of markers doesn't match the number of regions, it returns an error.
func parseRegionSpans(want string, regions []Region) (string, []Region, error) {
	type position struct {
		start  int
		length int
	}

	// Find all « » markers in order and record their positions
	var (
		positions []position
		cleaned   strings.Builder
	)
	for {
		// Opening marker.
		beforeOpen, afterOpen, found := strings.Cut(want, "«")
		if !found {
			// No more markers, consume the rest.
			cleaned.WriteString(want)
			break
		}
		cleaned.WriteString(beforeOpen)

		// Closing marker.
		label, afterClose, found := strings.Cut(afterOpen, "»")
		if !found {
			break
		}

		positions = append(positions, position{
			start:  cleaned.Len(),
			length: len(label),
		})

		cleaned.WriteString(label)
		want = afterClose
	}

	// Number of markers must match number of regions
	// or the test case is invalid.
	if len(positions) != len(regions) {
		return "", nil, fmt.Errorf("marker count mismatch: found %d markers but got %d regions",
			len(positions), len(regions))
	}

	var result []Region
	for i, r := range regions {
		r.Offset = positions[i].start
		r.Length = positions[i].length
		result = append(result, r)
	}

	return cleaned.String(), result, nil
}

// TestParseWant tests the parseWant helper function.
func TestParseWant(t *testing.T) {
	tests := []struct {
		desc        string
		want        string
		regions     []Region
		wantString  string
		wantOffsets []struct {
			offset int
			length int
		}
		wantErr bool
	}{
		{
			desc: "simple marker",
			want: "type Foo «Bar»",
			regions: []Region{
				{Label: &DeclLabel{Name: "Bar"}},
			},
			wantString: "type Foo Bar",
			wantOffsets: []struct {
				offset int
				length int
			}{
				{offset: 9, length: 3},
			},
		},
		{
			desc: "multiple markers",
			want: "«int» + «string»",
			regions: []Region{
				{Label: &EntityRefLabel{Name: "int"}},
				{Label: &EntityRefLabel{Name: "string"}},
			},
			wantString: "int + string",
			wantOffsets: []struct {
				offset int
				length int
			}{
				{offset: 0, length: 3},
				{offset: 6, length: 6},
			},
		},
		{
			desc:    "marker count mismatch (too few markers)",
			want:    "«int» and string",
			regions: []Region{{}, {}},
			wantErr: true,
		},
		{
			desc:    "marker count mismatch (too many markers)",
			want:    "«int» and «string»",
			regions: []Region{{}},
			wantErr: true,
		},
		{
			desc:       "no markers",
			want:       "type Foo",
			regions:    []Region{},
			wantString: "type Foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, gotRegions, err := parseRegionSpans(tt.want, tt.regions)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantString, got)
			require.Equal(t, len(tt.wantOffsets), len(gotRegions))
			for i, expected := range tt.wantOffsets {
				assert.Equal(t, expected.offset, gotRegions[i].Offset)
				assert.Equal(t, expected.length, gotRegions[i].Length)
			}
		})
	}
}

// func TestDeclFormatter_packageEntityRef(t *testing.T) {
// 	fset := token.NewFileSet()
// 	fname := filepath.Join("testdata", "package_importer.go")
// 	file, err := parser.ParseFile(fset, fname, nil, parser.ParseComments)
// 	require.NoError(t, err)

// 	ast.NewPackage(fset, map[string]*ast.File{
// 		fname: file,
// 	}, func(imports map[string]*ast.Object, path string) (pkg *ast.Object, err error) {
// 		if o, ok := imports[path]; ok {
// 			return o, nil
// 		}

// 		if path != "example.com/service-go" {
// 			return nil, fmt.Errorf("unknown import %q", path)
// 		}

// 		o := ast.NewObj(ast.Pkg, "service")
// 		o.Data = ast.NewScope(nil)
// 		imports[path] = o
// 		return o, nil
// 	}, nil)

// 	require.NotEmpty(t, file.Decls)

// 	df := NewDeclFormatter(fset, []string{"Handler"})
// 	df.Debug(true)
// 	src, gotRegions, err := df.FormatDecl(file.Decls[len(file.Decls)-1])
// 	require.NoError(t, err)

// 	assert.Empty(t, src)
// 	assert.Empty(t, gotRegions)
// }
