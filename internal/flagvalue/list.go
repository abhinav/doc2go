package flagvalue

import (
	"fmt"
	"strings"

	"braces.dev/errtrace"
)

// List is a generic flag.Getter
// that accepts zero or more instances of the same flag
// and combines them into a list.
type List[T any, PT Getter[T]] []T

// ListOf wraps a slice of flag.Getter objects
// to accept zero or more instances of that flag.
//
//	flag.Var(flagvalue.ListOf(&items), "item", ...)
func ListOf[T any, PT Getter[T]](vs *[]T) *List[T, PT] {
	return (*List[T, PT])(vs)
}

// Get returns the values recorded so far
// as a slice of the underlying type.
func (lv *List[T, PT]) Get() any { return []T(*lv) }

// String returns a semicolon separated list of the values in this list.
func (lv *List[T, PT]) String() string {
	var sb strings.Builder
	for i, v := range *lv {
		if i > 0 {
			sb.WriteString("; ")
		}
		fmt.Fprint(&sb, v)
	}
	return sb.String()
}

// Set receives a single flag argument into this list.
func (lv *List[T, PT]) Set(s string) error {
	var v T
	if err := PT(&v).Set(s); err != nil {
		return errtrace.Wrap(err)
	}
	*lv = append(*lv, v)
	return nil
}
