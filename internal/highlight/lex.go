package highlight

import (
	chroma "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// GoLexer is a [Lexer] that recognizes Go.
var GoLexer = &chromaLexer{l: chroma.Coalesce(lexers.Go)}

// Lexer analyzes source code and generates a stream of tokens.
type Lexer interface {
	Lex(src []byte) ([]chroma.Token, error)
}

// chromaLexer builds a [Lexer] from a Chroma lexer.
type chromaLexer struct{ l chroma.Lexer }

// Lex lexically analyzes the given source code using Chroma.
func (cl *chromaLexer) Lex(src []byte) ([]chroma.Token, error) {
	return chroma.Tokenise(cl.l, nil, string(src))
}
