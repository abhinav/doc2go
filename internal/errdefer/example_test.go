package errdefer_test

import (
	"io"
	"os"

	"go.abhg.dev/doc2go/internal/errdefer"
)

func readFile(name string) (_ []byte, err error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer errdefer.Close(&err, f)
	// NOTE: err must be a named return.

	return io.ReadAll(f)
}

// This is a contrived example
// but to demonstrate errdefer,
// we need a function that returns an error.
func ExampleClose() {
	_, err := readFile("example_test.go")
	if err != nil {
		panic(err)
	}
}
