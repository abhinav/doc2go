package sliceutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveCommonPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc         string
		giveA, giveB []int
		wantA, wantB []int
	}{
		{
			desc:  "empty/a",
			giveB: []int{1, 2, 3},
			wantB: []int{1, 2, 3},
		},
		{
			desc:  "empty/b",
			giveA: []int{1, 2, 3},
			wantA: []int{1, 2, 3},
		},
		{desc: "empty/both"},
		{
			desc:  "equal",
			giveA: []int{1, 2, 3},
			giveB: []int{1, 2, 3},
		},
		{
			desc:  "short a",
			giveA: []int{1, 2},
			giveB: []int{1, 2, 3, 4},
			wantB: []int{3, 4},
		},
		{
			desc:  "short b",
			giveA: []int{1, 2, 3, 4},
			giveB: []int{1, 2},
			wantA: []int{3, 4},
		},
		{
			desc:  "divergent",
			giveA: []int{1, 2, 3},
			giveB: []int{1, 2, 4},
			wantA: []int{3},
			wantB: []int{4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			gotA, gotB := RemoveCommonPrefix(tt.giveA, tt.giveB)
			assert.Equal(t, tt.wantA, gotA, "a")
			assert.Equal(t, tt.wantB, gotB, "b")
		})
	}
}
