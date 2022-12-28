package flagvalue

import (
	"flag"
	"io"
	"os"
)

// FileSwitch is a flag that accepts both "-x" and "-x=value",
// If a value is specified, it opens a file with that name.
// Otherwise, it uses a provided fallback writer.
type FileSwitch string

var _ flag.Getter = (*FileSwitch)(nil)

// Get returns the path stored in the writer
// or '-' if no value was specified.
func (fs *FileSwitch) Get() any { return string(*fs) }

// String returns the path stored in the writer
// or '-' if no value was specified.
func (fs *FileSwitch) String() string {
	return string(*fs)
}

// IsBoolFlag marks this as a flag
// that doesn't require a value.
func (*FileSwitch) IsBoolFlag() bool {
	return true
}

// Set receives the value for this flag.
func (fs *FileSwitch) Set(v string) error {
	if v == "true" {
		v = "-"
	}
	*fs = FileSwitch(v)
	return nil
}

// Bool reports whether this flag was set with any value.
func (fs *FileSwitch) Bool() bool {
	return len(*fs) > 0
}

// Create creates the file specified for this flag,
// and returns an io.Writer to it and a function to close it.
//
// This has three possible behaviors:
//
//   - the flag wasn't passed in: returns an [io.Discard]
//   - the flag was passed without a value: returns the provided fallback
//   - the flag was passed with a value: opens the file and returns it
func (fs *FileSwitch) Create(fallback io.Writer) (w io.Writer, close func() error, err error) {
	switch *fs {
	case "":
		return io.Discard, nopClose, nil
	case "-":
		return fallback, nopClose, nil
	default:
		f, err := os.Create(string(*fs))
		if err != nil {
			return nil, nil, err
		}
		return f, f.Close, nil
	}
}

func nopClose() error { return nil }
