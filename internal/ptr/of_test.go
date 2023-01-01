package ptr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "foo", *Of("foo"))
	assert.Equal(t, 42, *Of(42))
}
