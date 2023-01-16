package highlight

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

// PlainStyle is a minimal syntax highlighting style for Chroma.
// It leaves most text as-is, and fades comments ever so slightly.
var PlainStyle = chroma.MustNewStyle("plain", map[chroma.TokenType]string{
	chroma.Comment:    "#666666",
	chroma.PreWrapper: "bg:#eeeeee",
	chroma.Background: "bg:#eeeeee",
})

func init() {
	styles.Register(PlainStyle)
}
