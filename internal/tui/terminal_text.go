package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

func consumeCSI(s string, start int) int {
	i := start + 2
	for i < len(s) {
		c := s[i]
		i++
		if c >= '@' && c <= '~' {
			break
		}
	}
	return i
}

// stripANSI removes CSI escape sequences and zero-width terminal control
// characters while preserving user-visible Unicode text.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i = consumeCSI(s, i)
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		if r < 0x20 || r == 0x7f {
			i += size
			continue
		}
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

// cellWidth returns the display width used by a terminal. Wide CJK/emoji
// graphemes count as two cells and combining sequences do not consume an
// extra cell.
func cellWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

// truncateCells truncates styled terminal text to at most width display cells
// while preserving ANSI sequences before the cut. It evaluates the complete
// visible prefix so combining/ZWJ sequences use the same width rules as
// cellWidth. A visible ellipsis is used when truncation is required.
func truncateCells(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if cellWidth(s) <= width {
		return s
	}
	if width == 1 {
		return "…" + reset
	}

	var out strings.Builder
	var visible strings.Builder
	limit := width - 1
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			end := consumeCSI(s, i)
			out.WriteString(s[i:end])
			i = end
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		i += size
		if r < 0x20 || r == 0x7f {
			continue
		}

		candidate := visible.String() + string(r)
		if runewidth.StringWidth(candidate) > limit {
			break
		}
		visible.WriteRune(r)
		out.WriteRune(r)
	}
	out.WriteRune('…')
	out.WriteString(reset)
	return out.String()
}

func padRightCells(s string, width int) string {
	s = truncateCells(s, width)
	padding := width - cellWidth(s)
	if padding < 0 {
		padding = 0
	}
	return s + strings.Repeat(" ", padding)
}

func padLeftCells(s string, width int) string {
	s = truncateCells(s, width)
	padding := width - cellWidth(s)
	if padding < 0 {
		padding = 0
	}
	return strings.Repeat(" ", padding) + s
}

// cellIndex returns the zero-based display-cell index of the first visible
// occurrence of needle in s, or -1 when needle is absent.
func cellIndex(s, needle string) int {
	plain := stripANSI(s)
	plainNeedle := stripANSI(needle)
	byteIndex := strings.Index(plain, plainNeedle)
	if byteIndex < 0 {
		return -1
	}
	return runewidth.StringWidth(plain[:byteIndex])
}
