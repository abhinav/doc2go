package flagvalue

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSwitch_NoArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give []string

		wantGet    string
		wantString string
		wantBool   bool
	}{
		{
			desc:     "no argument",
			wantBool: false,
		},
		{
			desc:       "default argument",
			give:       []string{"-x"},
			wantGet:    "-",
			wantString: "-",
			wantBool:   true,
		},
		{
			desc:       "explicit argument",
			give:       []string{"-x=foo"},
			wantGet:    "foo",
			wantString: "foo",
			wantBool:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
			var fs FileSwitch
			fset.Var(&fs, "x", "")
			require.NoError(t, fset.Parse(tt.give))

			assert.Equal(t, tt.wantGet, fs.Get())
			assert.Equal(t, tt.wantString, fs.String())
			assert.Equal(t, tt.wantBool, fs.Bool())
		})
	}
}

func TestFileSwitch_Create(t *testing.T) {
	t.Parallel()

	parse := func(t *testing.T, args ...string) *FileSwitch {
		fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
		var fs FileSwitch
		fset.Var(&fs, "x", "")
		require.NoError(t, fset.Parse(args))
		return &fs
	}

	t.Run("no arguments", func(t *testing.T) {
		t.Parallel()

		fs := parse(t)

		got, done, err := fs.Create(new(bytes.Buffer))
		require.NoError(t, err)
		assert.True(t, got == io.Discard, "expected io.Discard, got %v", got)
		require.NoError(t, done())
	})

	t.Run("fallback", func(t *testing.T) {
		t.Parallel()

		fs := parse(t, "-x")
		buff := new(bytes.Buffer)

		got, done, err := fs.Create(buff)
		require.NoError(t, err)
		assert.True(t, got == buff, "expected io.Discard, got %v", got)
		require.NoError(t, done())
	})

	t.Run("explicit", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "foo.txt")
		fs := parse(t, "-x="+path)

		got, done, err := fs.Create(new(bytes.Buffer))
		require.NoError(t, err)
		_, err = io.WriteString(got, "hello")
		require.NoError(t, err)
		require.NoError(t, done())

		body, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(body))
	})
}

func TestFileSwitch_Create_error(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "does_not_exist", "foo.txt")
	fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)

	var fs FileSwitch
	fset.Var(&fs, "x", "")
	require.NoError(t, fset.Parse([]string{"-x=" + path}))

	_, _, err := fs.Create(new(bytes.Buffer))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
