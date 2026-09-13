package tui

import (
	"strings"
	"unicode/utf8"
)

// stripANSI removes CSI escape sequences and zero-width terminal control
// characters. The current renderer is intentionally rune-cell oriented: ASCII
// and the box-drawing characters used by nasmon are treated as one cell each.
// If wide/combining Unicode becomes a real input requirement, this is the one
// layer to replace with a display-width implementation such as go-runewidth.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) {
				c := s[i]
				i++
				if c >= '@' && c <= '~' {
					break
				}
			}
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

// cellWidth returns the terminal-cell width used by nasmon's current
// ASCII-oriented renderer. ANSI/control bytes are zero-width and every visible
// rune counts as one cell.
func cellWidth(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

// truncateCells truncates styled terminal text to at most width cells while
// preserving ANSI sequences that occur before the cut. A visible ellipsis is
// used when truncation is required.
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

	var b strings.Builder
	visible := 0
	limit := width - 1
	for i := 0; i < len(s) && visible < limit; {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			start := i
			i += 2
			for i < len(s) {
				c := s[i]
				i++
				if c >= '@' && c <= '~' {
					break
				}
			}
			b.WriteString(s[start:i])
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
		b.WriteRune(r)
		visible++
	}
	b.WriteRune('…')
	b.WriteString(reset)
	return b.String()
}

// cellIndex returns the zero-based cell index of the first visible occurrence
// of needle in s, or -1 when needle is absent. ANSI/control bytes do not affect
// the result.
func cellIndex(s, needle string) int {
	plain := stripANSI(s)
	plainNeedle := stripANSI(needle)
	byteIndex := strings.Index(plain, plainNeedle)
	if byteIndex < 0 {
		return -1
	}
	return utf8.RuneCountInString(plain[:byteIndex])
}
