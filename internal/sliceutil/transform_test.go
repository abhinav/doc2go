package sliceutil

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give []int
		fn   func(int) string
		want []string
	}{
		{
			desc: "empty",
			fn:   strconv.Itoa,
		},
		{
			desc: "non-empty",
			give: []int{1, 2, 3, 4},
			fn:   strconv.Itoa,
			want: []string{"1", "2", "3", "4"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got := Transform(tt.give, tt.fn)
			assert.Equal(t, tt.want, got)
		})
	}
}
