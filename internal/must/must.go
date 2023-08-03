// Package must provides helper functions to assert program invariants.
// The program will panic if an invariant is violated.
package must

import "fmt"

// panicf panics with the printf-style message.
func panicf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

// NotErrorf panics with the given message if the error is not nil.
func NotErrorf(err error, format string, args ...interface{}) {
	if err != nil {
		panicf("unexpected error: %v\n%v", err, fmt.Sprintf(format, args...))
	}
}
