// Package iotest provides utilities for testing IO-related code.
package iotest

import (
	"io"
	"testing"

	"go.abhg.dev/doc2go/internal/linebuf"
)

// Writer builds and returns an io.Writer that
// writes messages to the given testing.TB.
// It ensures that each line is logged separately.
//
// Any trailing buffered text that does not end with a newline
// is flushed when the test finishes.
//
// The returned writer is safe for concurrent use
// from multiple parallel tests.
func Writer(t testing.TB) io.Writer {
	w, done := linebuf.Writer(func(line []byte) {
		t.Helper()

		// Strip trailing newline, if any;
		// t.Logf adds its own newline.
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}

		t.Logf("%s", line)
	})
	t.Cleanup(done)
	return w
}
