package must

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotErrorf(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		NotErrorf(nil, "should not panic")
	})

	t.Run("not-nil", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			NotErrorf(errors.New("error"), "should panic")
		})
	})
}
