// Package errdefer provides functions for running operations
// that must be deferred until the end of a function,
// but which may return errors that should be returned from the function.
package errdefer

import (
	"errors"
	"io"
)

// Close calls Close on the given Closer,
// and joins any error returned with the given error.
//
// Use it inside a defer statement with a named return like this:
//
//	func foo() (err error) {
//		f, err := os.Open("foo.txt")
//		if err != nil {
//			return err
//		}
//		defer errdefer.Close(&err, f)
//		// ...
//	}
func Close(err *error, closer io.Closer) {
	*err = errors.Join(*err, closer.Close())
}
