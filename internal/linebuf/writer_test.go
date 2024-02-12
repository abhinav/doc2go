package linebuf

import (
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string

		writes []string // individual write calls
		want   []string // expected log output
	}{
		{
			desc:   "empty strings",
			writes: []string{"", "", ""},
		},
		{
			desc:   "no newline",
			writes: []string{"foo", "bar", "baz"},
			want:   []string{"foobarbaz"},
		},
		{
			desc: "newline separated",
			writes: []string{
				"foo\n",
				"bar\n",
				"baz\n\n",
				"qux",
			},
			want: []string{
				"foo\n",
				"bar\n",
				"baz\n",
				"\n",
				"qux",
			},
		},
		{
			desc:   "partial line",
			writes: []string{"foo", "bar\nbazqux"},
			want: []string{
				"foobar\n",
				"bazqux",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var got []string
			w, done := Writer(func(line []byte) {
				got = append(got, string(line))
			})

			for _, input := range tt.writes {
				n, err := w.Write([]byte(input))
				assert.NoError(t, err)
				assert.Equal(t, len(input), n)
			}

			done()

			assert.Equal(t, tt.want, got)
		})
	}
}

// Ensures that there are no data races in Writer
// by writing to it from multiple concurrent goroutines.
// 'go test -race' will explode if there's a data race.
func TestWriterRace(t *testing.T) {
	t.Parallel()

	const N = 100 // number of concurrent writers

	var numWrites int
	w, done := Writer(func([]byte) {
		// We don't care about the contents of the line.
		// If there's a race, the increment will trip test -race.
		numWrites++
	})
	defer done()

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()

			_, err := io.WriteString(w, "foo\n")
			require.NoError(t, err)
			_, err = io.WriteString(w, "bar\n")
			require.NoError(t, err)
			_, err = io.WriteString(w, "baz\n")
			require.NoError(t, err)
		}()
	}
	wg.Wait()
}
