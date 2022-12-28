package slices

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveFunc(t *testing.T) {
	type skipfn func(int) bool
	always := func(b bool) skipfn { return func(int) bool { return b } }

	bools := func(bs ...bool) skipfn {
		idx := 0
		return func(int) bool {
			b := bs[idx]
			idx++
			return b
		}
	}

	tests := []struct {
		desc string
		give []int
		skip skipfn
		want []int
	}{
		{
			desc: "empty",
			skip: always(true),
		},
		{
			desc: "skip all",
			give: []int{1, 2, 3},
			skip: always(true),
			want: []int{},
		},
		{
			desc: "skip none",
			give: []int{1, 2, 3},
			skip: always(false),
			want: []int{1, 2, 3},
		},
		{
			desc: "skip some",
			give: []int{1, 2, 3, 4, 5},
			skip: bools(true, false, true, true, false),
			want: []int{2, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := RemoveFunc(tt.give, tt.skip)
			assert.Equal(t, tt.want, got)
		})
	}
}
