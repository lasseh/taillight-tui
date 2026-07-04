package highlight

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Truncate shortens s to at most maxWidth terminal cells, appending "..."
// when content is cut. Widths are measured in display cells, so multi-byte
// and wide runes (ø, emoji, CJK) are never split mid-character.
func Truncate(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return ansi.Truncate(s, maxWidth, "")
	}
	return ansi.Truncate(s, maxWidth, "...")
}

// PadRight pads s with spaces to exactly width display cells, truncating
// (without ellipsis) when it is wider.
func PadRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = ansi.Truncate(s, width, "")
	if w := ansi.StringWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
