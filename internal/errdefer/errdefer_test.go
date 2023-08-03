package errdefer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClose(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		var err error
		Close(&err, stubCloser{})
		assert.NoError(t, err)
	})

	t.Run("non-nil", func(t *testing.T) {
		t.Parallel()

		give := errors.New("sadness")

		var err error
		Close(&err, stubCloser{err: give})
		assert.ErrorIs(t, err, give)
	})
}

type stubCloser struct {
	err error
}

func (s stubCloser) Close() error {
	return s.err
}
