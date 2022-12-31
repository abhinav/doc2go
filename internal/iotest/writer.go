package iotest

import (
	"bytes"
	"io"
	"testing"
)

var _newline = []byte("\n")

// Writer builds an io.Writer that writes to the given testing.TB.
func Writer(t testing.TB) io.Writer {
	return &writer{t}
}

type writer struct{ t testing.TB }

func (w *writer) Write(b []byte) (int, error) {
	b = bytes.TrimSuffix(b, _newline)
	w.t.Logf("%s", b)
	return len(b), nil
}
