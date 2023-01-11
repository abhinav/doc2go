package pathx

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescends(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b string
		want bool
	}{
		{"foo", "bar", false},
		{"foo", "foobar", false},
		{"foo", "foo/bar", true},
		{"foo/", "foo/bar", true},
		{"foo/", "foobar", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("Descends(%q,%q)", tt.a, tt.b), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, Descends(tt.a, tt.b))
		})
	}
}
