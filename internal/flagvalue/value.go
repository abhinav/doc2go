// Package flagvalue provides flag.Value implementations.
package flagvalue

import "flag"

// Getter is a constraint satisfied by pointers to types
// which implement flag.Getter.
type Getter[T any] interface {
	*T
	flag.Getter
}
