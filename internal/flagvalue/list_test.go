package flagvalue

import (
	"errors"
	"flag"
	"io"
	"testing"

	"braces.dev/errtrace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stringValue string

var _ flag.Getter = (*stringValue)(nil)

func (sv *stringValue) Get() any       { return sv.String() }
func (sv *stringValue) String() string { return string(*sv) }
func (sv *stringValue) Set(s string) error {
	*sv = stringValue(s)
	return nil
}

func TestList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		give       []string
		want       []stringValue
		wantString string
	}{
		{
			desc: "no arguments",
			give: []string{"-y"},
		},
		{
			desc:       "separate",
			give:       []string{"-x", "foo"},
			want:       []stringValue{"foo"},
			wantString: "foo",
		},
		{
			desc:       "joint",
			give:       []string{"-x=foo"},
			want:       []stringValue{"foo"},
			wantString: "foo",
		},
		{
			desc:       "multiple",
			give:       []string{"-x", "foo", "-x=bar"},
			want:       []stringValue{"foo", "bar"},
			wantString: "foo; bar",
		},
		{
			desc:       "interleaved",
			give:       []string{"-x", "foo", "-y", "-x=bar"},
			want:       []stringValue{"foo", "bar"},
			wantString: "foo; bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)

			var got []stringValue
			list := ListOf(&got)
			fset.Var(list, "x", "")
			_ = fset.Bool("y", false, "")
			require.NoError(t, fset.Parse(tt.give))

			assert.Equal(t, tt.want, got)

			assert.Equal(t, tt.want, list.Get(), "Get")
			assert.Equal(t, tt.wantString, list.String(), "String")
		})
	}
}

type fallibleStringValue string

var _ flag.Getter = (*fallibleStringValue)(nil)

func (sv *fallibleStringValue) Get() any       { return sv.String() }
func (sv *fallibleStringValue) String() string { return string(*sv) }

func (sv *fallibleStringValue) Set(s string) error {
	if s == "fail" {
		return errtrace.Wrap(errors.New("great sadness"))
	}
	*sv = fallibleStringValue(s)
	return nil
}

func TestList_error(t *testing.T) {
	t.Parallel()

	fset := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
	fset.SetOutput(io.Discard)

	var got []fallibleStringValue
	fset.Var(ListOf(&got), "x", "")

	err := fset.Parse([]string{"-x=foo", "-x=fail", "-x", "bar"})
	assert.ErrorContains(t, err, "great sadness")
}
