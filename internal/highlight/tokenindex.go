package highlight

import (
	"sort"

	chroma "github.com/alecthomas/chroma/v2"
)

// TokenIndex is a searchable collection of tokens.
type TokenIndex struct {
	src    []byte
	tokens []chroma.Token
	starts []int // start offset in src of tokens[i]
	ends   []int // end offset in src of tokens[i]
}

// NewTokenIndex builds a token index given source code and its tokens.
func NewTokenIndex(src []byte, tokens []chroma.Token) *TokenIndex {
	starts := make([]int, len(tokens))
	ends := make([]int, len(tokens))
	for i, t := range tokens {
		var start int
		if i > 0 {
			start = ends[i-1]
		}
		starts[i] = start
		ends[i] = start + len(t.Value)
	}

	return &TokenIndex{
		src:    src,
		tokens: tokens,
		starts: starts,
		ends:   ends,
	}
}

// Interval returns a list of tokens that are in the range [start, end).
// If the token boundaries aren't exactly matching,
// intervals also returns the leading and trailing text, if any.
func (ts *TokenIndex) Interval(start, end int) (tokens []chroma.Token, lead, trail []byte) {
	startIdx := sort.SearchInts(ts.starts, start)
	if startIdx >= len(ts.starts) {
		return nil, nil, nil
	}

	// starts[startIdx] is equal to start if it's a token boundary,
	// or greater than start if start is in the middle of a token.
	// That's the leading text.
	if off := ts.starts[startIdx]; start < off {
		lead = ts.src[start:off]
	}

	endIdx := sort.SearchInts(ts.ends[startIdx:], end) + startIdx
	if endIdx >= len(ts.ends) {
		return ts.tokens[startIdx:], lead, trail
	}

	// ends[endIdx] is greater than end if end is in the middle of a token.
	// In that case, we'll drop that token but take text up to end
	// as trailing text.
	if off := ts.ends[endIdx]; end < off {
		if ts.starts[endIdx] < end {
			trail = ts.src[ts.starts[endIdx]:end]
		}
	} else {
		// If ends[endIdx] is equal to end, though,
		// that token must be included in its entirety.
		endIdx++
	}

	return ts.tokens[startIdx:endIdx], lead, trail
}
