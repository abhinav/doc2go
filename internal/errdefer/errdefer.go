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
// Use it inside a defer statement with a named return.
func Close(err *error, closer io.Closer) {
	*err = errors.Join(*err, closer.Close())
}
