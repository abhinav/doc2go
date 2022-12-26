// Package pathtree provides a data structure
// that stores values organized under a tree-like hierarchy
// where values from higher levels cascade down to lower levels
// unless the lower levels define their own values.
//
// For example, if 'foo/bar' defines a value X,
// foo/bar, foo/bar/baz, and foo/bar/qux and all their descendants
// inherit this value.
//
//	t.Set("foo/bar", X)
//	t.Get("foo/bar")     // == X
//	t.Get("foo/bar/baz") // == X
//	t.Get("foo/bar/qux") // == X
//
// However, if 'foo/bar/baz' defines a different value Y,
// it and its descendants use that value.
//
//	t.Set("foo/bar",     X)
//	t.Set("foo/bar/baz", Y)
//	t.Get("foo/bar")         // == X
//	t.Get("foo/bar/qux")     // == X
//	t.Get("foo/bar/baz")     // == Y
//	t.Get("foo/bar/baz/qux") // == Y
package pathtree

import (
	"sort"
	"strings"
)

const _sep = '/'

// Root is the starting point of the path tree.
// The zero-value of Root is an empty tree.
type Root[T any] struct {
	root node[T]
}

// Set adds a value to the tree under the given path.
// All descendants of this path that do not have an explicit value
// will inherit this value.
// If this path already had a value specified, it will be overwritten.
func (r *Root[T]) Set(p string, v T) {
	r.root.Set(p, &v)
}

// Lookup retrieves the value for the given path,
// inheriting values specified for parents of this path
// if it didn't get its own value.
//
// Lookup reports true if a value was found--even if it was inherited.
func (r *Root[T]) Lookup(p string) (v T, ok bool) {
	if got := r.root.Get(p, nil); got != nil {
		v = *got
		ok = true
	}
	return v, ok
}

// Snapshot is a snapshot of values added to the tree
// presented in a hierarchical manner.
type Snapshot[T any] struct {
	// Value in the tree,
	// or nil if this node doesn't have an explicit value.
	Value *T
	// Path to this node.
	Path string
	// Children of this node.
	Children []Snapshot[T]
}

// Snapshot builds and returns a snapshot of all values
// in this path tree.
//
// The returned slice holds nodes closest to root.
func (r *Root[T]) Snapshot() []Snapshot[T] {
	return r.root.Snapshot(nil).Children
}

type node[T any] struct {
	value *T
	// TODO: children should be a sorted list that we binary search inside.
	children map[string]*node[T]
}

func (n *node[T]) ensurechild(name string) *node[T] {
	if n.children == nil {
		n.children = make(map[string]*node[T])
	}

	c, ok := n.children[name]
	if !ok {
		c = new(node[T])
		n.children[name] = c
	}
	return c
}

func (n *node[T]) Set(p string, v *T) {
	if len(p) == 0 {
		n.value = v
		return
	}

	head, tail := split(p)
	n.ensurechild(head).Set(tail, v)
}

func (n *node[T]) Get(p string, current *T) (final *T) {
	if n == nil {
		return current
	}

	if n.value != nil {
		current = n.value
	}

	head, tail := split(p)
	return n.children[head].Get(tail, current)
}

func (n *node[T]) Snapshot(path []string) Snapshot[T] {
	var children []Snapshot[T]
	if len(n.children) > 0 {
		childNames := make([]string, 0, len(n.children))
		for name := range n.children {
			childNames = append(childNames, name)
		}
		// TODO: This won't be necessary once node.children
		// is turned into a sorted slice.
		sort.Strings(childNames)

		children = make([]Snapshot[T], len(childNames))
		for i, name := range childNames {
			children[i] = n.children[name].Snapshot(append(path, name))
		}
	}

	return Snapshot[T]{
		Value:    n.value,
		Path:     strings.Join(path, string(_sep)),
		Children: children,
	}
}

func split(p string) (head, tail string) {
	head, tail = p, ""
	if idx := strings.IndexByte(p, _sep); idx >= 0 {
		head, tail = p[:idx], p[idx+1:]
	}
	// If tail has any extra slashes, at the start, get rid of them.
	for len(tail) > 0 && tail[0] == _sep {
		tail = tail[1:]
	}
	return head, tail
}
