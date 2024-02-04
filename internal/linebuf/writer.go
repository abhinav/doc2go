// Package linebuf provides line-buffered IO utilities.
package linebuf

import (
	"bytes"
	"io"
	"sync"
)

// Writer return san io.Writer that splits its input on newline,
// calling fn for each line -- including the trailing newline.
func Writer(fn func([]byte)) (_ io.Writer, done func()) {
	w := writer{writeLine: fn}
	return &w, w.flush
}

// writer is an io.Writer that writes to a testing.T.
type writer struct {
	writeLine func([]byte)

	// Holds buffered text for the next write or flush
	// if we haven't yet seen a newline.
	buff bytes.Buffer
	mu   sync.Mutex // guards buff
}

func (w *writer) Write(bs []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// t.Logf adds a newline so we should not write bs as-is.
	// Instead, we'll call t.Log one line at a time.
	//
	// To handle the case when Write is called with a partial line,
	// we use a buffer.
	total := len(bs)
	for len(bs) > 0 {
		idx := bytes.IndexByte(bs, '\n')
		if idx < 0 {
			// No newline. Buffer it for later.
			w.buff.Write(bs)
			break
		}

		var line []byte
		line, bs = bs[:idx+1], bs[idx+1:]

		if w.buff.Len() == 0 {
			// Nothing buffered from a prior partial write.
			// This is the majority case.
			w.writeLine(line)
			continue
		}

		// There's a prior partial write. Join and flush.
		w.buff.Write(line)
		w.writeLine(w.buff.Bytes())
		w.buff.Reset()
	}
	return total, nil
}

// flush flushes buffered text, even if it doesn't end with a newline.
func (w *writer) flush() {
	if w.buff.Len() > 0 {
		w.writeLine(w.buff.Bytes())
	}
}
