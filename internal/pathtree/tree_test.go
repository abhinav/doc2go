package pathtree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.abhg.dev/doc2go/internal/ptr"
)

func TestEmpty(t *testing.T) {
	t.Parallel()

	var r Root[int]
	_, ok := r.Lookup("foo")
	require.False(t, ok)

	t.Run("snapshot", func(t *testing.T) {
		assert.Empty(t, r.Snapshot())
	})
}

func TestSetAndGet(t *testing.T) {
	t.Parallel()

	ensure := ensurer[int](t)

	var r Root[int]
	r.Set("foo", 42)

	assert.Equal(t, 42, ensure(r.Lookup("foo")), "exact")
	assert.Equal(t, 42, ensure(r.Lookup("foo/bar")), "child")
	assert.Equal(t, 42, ensure(r.Lookup("foo/bar/baz/qux/quux")), "descendant")

	t.Run("sibling", func(t *testing.T) {
		t.Parallel()

		_, ok := r.Lookup("foobar")
		require.False(t, ok)
	})

	t.Run("snapshot", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []Snapshot[int]{
			{
				Path:  "foo",
				Value: ptr.Of(42),
			},
		}, r.Snapshot())
	})
}

func TestExtraneousSlashes(t *testing.T) {
	t.Parallel()

	ensure := ensurer[int](t)

	var r Root[int]
	r.Set("foo", 42)
	r.Set("foo////bar", 43)

	assert.Equal(t, 42, ensure(r.Lookup("foo")))
	assert.Equal(t, 42, ensure(r.Lookup("foo/foo")))
	assert.Equal(t, 42, ensure(r.Lookup("foo/////foo")))

	assert.Equal(t, 43, ensure(r.Lookup("foo/bar")))
	assert.Equal(t, 43, ensure(r.Lookup("foo///bar/baz")))
	assert.Equal(t, 43, ensure(r.Lookup("foo/bar///baz")))
}

func TestDescendantOverride(t *testing.T) {
	t.Parallel()

	ensure := ensurer[int](t)

	var r Root[int]
	r.Set("foo", 42)

	require.Equal(t, 42, ensure(r.Lookup("foo/bar/baz/qux/quux")), "descendant",
		"sanity check")

	r.Set("foo/bar/baz", 43)
	assert.Equal(t, 43, ensure(r.Lookup("foo/bar/baz/qux/quux")), "override")
	assert.Equal(t, 42, ensure(r.Lookup("foo/bar/quux")), "sibling")

	t.Run("snapshot", func(t *testing.T) {
		assert.Equal(t, []Snapshot[int]{
			{
				Path:  "foo",
				Value: ptr.Of(42),
				Children: []Snapshot[int]{
					{
						Path: "foo/bar",
						Children: []Snapshot[int]{
							{
								Path:  "foo/bar/baz",
								Value: ptr.Of(43),
							},
						},
					},
				},
			},
		}, r.Snapshot())
	})
}

func ensurer[T any](t *testing.T) func(T, bool) T {
	return func(v T, ok bool) T {
		t.Helper()

		require.True(t, ok)
		return v
	}
}
