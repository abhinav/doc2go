package gosrc

import (
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var formatExampleTests = []struct {
	name string
	give []string
	want []string
}{
	{
		name: "not a block",
		give: []string{
			"package main",
			"func main() {",
			`	fmt.Println("Hello, world!")`,
			"}",
		},
		want: []string{
			"package main",
			"func main() {",
			`	fmt.Println("Hello, world!")`,
			"}",
		},
	},
	{
		name: "empty block",
		give: []string{
			"{",
			"}",
		},
		want: []string{
			"",
		},
	},
	{
		name: "single line block",
		give: []string{
			"{",
			`	fmt.Println("Hello, world!")`,
			"}",
		},
		want: []string{
			`fmt.Println("Hello, world!")`,
		},
	},
	{
		name: "multiple newlines after block",
		give: []string{
			"{",
			"",
			`	fmt.Println("Hello, world!")`,
			"}",
		},
		want: []string{
			`fmt.Println("Hello, world!")`,
		},
	},
	{
		name: "multi line block",
		give: []string{
			"{",
			`	err := foo()`,
			`	if err != nil {`,
			`		return err`,
			`	}`,
			"}",
		},
		want: []string{
			`err := foo()`,
			`if err != nil {`,
			`	return err`,
			`}`,
		},
	},
	{
		name: "comments",
		give: []string{
			"{",
			`	// foo`,
			`	// bar`,
			`	// baz`,
			"}",
		},
		want: []string{
			`// foo`,
			`// bar`,
			`// baz`,
		},
	},
	{
		name: "block comment",
		give: []string{
			"{",
			`	/*`,
			`	foo`,
			`		bar`,
			`	baz`,
			`	*/`,
			"}",
		},
		want: []string{
			`/*`,
			`foo`,
			`	bar`,
			`baz`,
			`*/`,
		},
	},
	{
		name: "multi-line string literal",
		give: []string{
			"{",
			"	x := `",
			"	foo",
			"		bar",
			"	baz`",
			"}",
		},
		want: []string{
			"x := `",
			"	foo",
			"		bar",
			"	baz`",
		},
	},
	{
		name: "output comment",
		give: []string{
			"{",
			`	fmt.Println("foo")`,
			"",
			`	// Output:`,
			`	// foo`,
			`	// bar`,
			"}",
		},
		want: []string{
			`fmt.Println("foo")`,
		},
	},
	{
		name: "unordered output comment",
		give: []string{
			"{",
			`	fmt.Println("foo")`,
			`	fmt.Println("bar")`,
			"",
			`	// Unordered output:`,
			`	// bar`,
			`	// foo`,
			"}",
		},
		want: []string{
			`fmt.Println("foo")`,
			`fmt.Println("bar")`,
		},
	},
	{
		// This will not happen in practice because go/doc handles it,
		// but it's worth guarding against.
		name: "full file example output",
		give: []string{
			"package main",
			"",
			`import "fmt"`,
			"",
			"func main() {",
			`	fmt.Println("Hello, world!")`,
			"",
			`	// Output:`,
			`	// Hello, world!`,
			"}",
		},
		want: []string{
			"package main",
			"",
			`import "fmt"`,
			"",
			"func main() {",
			`	fmt.Println("Hello, world!")`,
			"}",
		},
	},
}

func TestFormatExample(t *testing.T) {
	for _, tt := range formatExampleTests {
		t.Run(tt.name, func(t *testing.T) {
			give := strings.Join(tt.give, "\n")
			assert.Equal(t,
				strings.Join(tt.want, "\n"),
				string(FormatExample([]byte(give))),
			)
		})
	}
}

func FuzzFormatExample(f *testing.F) {
	for _, tt := range formatExampleTests {
		if slices.Equal(tt.give, tt.want) {
			// Ignore tests that don't involve transformation.
			continue
		}

		f.Add(strings.Join(tt.give, "\n"))
	}

	f.Fuzz(func(t *testing.T, give string) {
		// The before and after should have the same tokens
		// minus the braces and comments.
		startToks, err := tokens(give)
		if err != nil {
			t.Skip()
		}

		switch {
		case len(startToks) < 2,
			startToks[0].tok != token.LBRACE,
			startToks[len(startToks)-1].tok != token.RBRACE:
			t.Skip()
		}

		wantToks := startToks[1 : len(startToks)-1]
		if len(wantToks) == 0 {
			wantToks = nil // empty slice != nil
		}

		got := FormatExample([]byte(give))
		gotToks, err := tokens(string(got))
		assert.NoError(t, err)
		assert.Equal(t, wantToks, gotToks)

		if t.Failed() {
			t.Logf("give:\n%s", give)
			t.Logf("got:\n%s", got)
		}
	})
}

type scanToken struct {
	tok token.Token
	lit string
}

func (t scanToken) GoString() string {
	return fmt.Sprintf("scanToken{%v, %q}", t.tok.String(), t.lit)
}

func (t scanToken) String() string {
	if len(t.lit) == 0 {
		return t.tok.String()
	}
	return fmt.Sprintf("%v(%q)", t.tok, t.lit)
}

func tokens(src string) ([]scanToken, error) {
	fset := token.NewFileSet()
	var scan scanner.Scanner
	file := fset.AddFile("", fset.Base(), len(src))
	scan.Init(file, []byte(src), nil /* ignore errors */, 0)

	var (
		toks  []scanToken
		depth int
	)
loop:
	for {
		pos, tok, lit := scan.Scan()
		switch tok {
		case token.EOF:
			break loop
		case token.LBRACE:
			depth++
		case token.RBRACE:
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unmatched brace at %v", file.Offset(pos))
			}
			if depth == 0 {
				// There shouldn't be any more tokens after this.
				pos, tok, _ := scan.Scan()
				if tok != token.EOF {
					return nil, fmt.Errorf("unexpected token %v at %v", tok, file.Offset(pos))
				}
			}
		case token.COMMENT, token.SEMICOLON:
			continue // ignore comments and statement delimiters
		}
		toks = append(toks, scanToken{tok, lit})
		file.Offset(pos)
	}

	if depth > 0 {
		return nil, errors.New("file ended before closing brace")
	}

	return toks, nil
}
