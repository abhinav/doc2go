package relative

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath(t *testing.T) {
	tests := []struct {
		desc string
		src  string
		dst  string
		want string
	}{
		{
			desc: "child",
			src:  "foo/bar",
			dst:  "foo/bar/baz/qux",
			want: "baz/qux",
		},
		{
			desc: "sibling",
			src:  "foo/bar/baz/qux",
			dst:  "foo/bar/baz/quux",
			want: "../quux",
		},
		{
			desc: "parent",
			src:  "foo/bar/baz/qux",
			dst:  "foo/bar",
			want: "../..",
		},
		{
			desc: "cousin",
			src:  "foo/bar/baz/qux/quux",
			dst:  "foo/a/b/c/d/e",
			want: "../../../../a/b/c/d/e",
		},
		{
			desc: "absolute",
			src:  "/foo/bar/baz",
			dst:  "/a/b/c",
			want: "../../../a/b/c",
		},
		{
			desc: "trailing slash src",
			src:  "foo/bar/",
			dst:  "foo/baz/qux",
			want: "../baz/qux",
		},
		{
			desc: "trailing slash both",
			src:  "foo/bar/",
			dst:  "foo/baz/qux/",
			want: "../baz/qux/",
		},
		{
			desc: "root",
			src:  "foo/bar/baz",
			dst:  "",
			want: "../../..",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := Path(tt.src, tt.dst)
			assert.Equal(t, tt.want, got)
		})
	}
}
