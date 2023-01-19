package main

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelp_Write(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give    Help
		wantErr string
	}{
		{give: "usage"},
		{give: "default"},
		{give: "frontmatter"},
		{give: "pkg-doc"},
		{
			give:    "not-a-topic",
			wantErr: `unknown help topic "not-a-topic": valid values`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.give.String(), func(t *testing.T) {
			t.Parallel()

			err := tt.give.Write(io.Discard)
			if len(tt.wantErr) > 0 {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
