package gosrc

import (
	"bytes"
	"go/scanner"
	"go/token"
	"regexp"
)

var (
	_newline  = []byte("\n")
	_outputRx = regexp.MustCompile(`(?i)//\s*(unordered )?output:`)
)

// TODO: move to godoc package?

// FormatExample prepares an example's source code
// for presentation in documentation.
// To do this, it performs a couple transformations.
//
// If the example is not a full-file example,
// it's likely wrapped in a BlockStmt -- with a '{' and '}'.
// This function removes those braces and unindent the code inside.
//
// Secondly, if the expected output is included
// in a comment at the end of the example,
// this function removes that comment and any blank lines before it.
// This is done because the output is already included separately
// in the rendered documentation.
func FormatExample(bs []byte) []byte {
	fset := token.NewFileSet()
	var scan scanner.Scanner
	file := fset.AddFile("", fset.Base(), len(bs))
	scan.Init(file, bs, nil /* ignore errors */, scanner.ScanComments)

	// No-op by default.
	unindent := func(bs []byte) []byte { return bs }

	pos, tok, lit := scan.Scan()
	var lastOffset int
	if tok == token.LBRACE {
		// If it doesn't start with a brace,
		// it doesn't need to be unindented.
		//
		// Use the offset between the opening brace
		// and the first real token to get the indentation string.
		indentStart := file.Offset(pos) + 1 // skip the '{'
		pos, tok, lit = scan.Scan()
		lastOffset = file.Offset(pos)
		indent := bs[indentStart:lastOffset]

		// Trim all but the last leading newline from indent.
		for i := 1; i < len(indent); i++ {
			if indent[i] != '\n' {
				indent = indent[i-1:]
				break
			}
		}

		// "\n\t" => "\n" removes the indent.
		if len(indent) > 0 && indent[0] == '\n' {
			unindent = func(bs []byte) []byte {
				return bytes.ReplaceAll(bs, indent, _newline)
			}
		}
	}

	// Scan through the rest of the source,
	// unindenting as needed.
	depth := 1
	out := make([]byte, 0, len(bs))
loop:
	for {
		switch tok {
		case token.EOF:
			break loop

		case token.LBRACE:
			depth++

		case token.RBRACE:
			depth--
			if depth == 0 {
				// Reached the end of the block.
				offset := file.Offset(pos)
				out = append(out, unindent(bs[lastOffset:offset])...)
				break loop
			}

		case token.COMMENT:
			// If the comment is an output comment,
			// skip it and any adjacent comments.
			if _outputRx.MatchString(lit) {
				var commentEndOffset int
				for tok == token.COMMENT {
					commentEndOffset = file.Offset(pos) + len(lit)
					pos, tok, lit = scan.Scan()
				}
				lastOffset = commentEndOffset

				// We've already written the spaces before the comment,
				// so trim any trailing whitespace.
				for i := len(out) - 1; i >= 0; i-- {
					if out[i] != '\n' && out[i] != '\t' && out[i] != ' ' {
						out = out[:i+1]
						break
					}
				}

				continue loop
			}

			// Otherwise, unindent and copy it.
			lastOffset = file.Offset(pos) + len(lit)
			out = append(out, unindent([]byte(lit))...)

		case token.STRING, token.CHAR:
			// Copy string literals verbatim,
			// including multi-line string literals.
			lastOffset = file.Offset(pos) + len(lit)
			out = append(out, lit...)
		}

		// Scan until next token and copy the text between --
		// minus the indent.
		pos, tok, lit = scan.Scan()
		offset := file.Offset(pos)
		out = append(out, unindent(bs[lastOffset:offset])...)
		lastOffset = offset

	}

	out = bytes.TrimRight(out, "\n")
	return out
}
