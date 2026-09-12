package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func fanRPMText(rpm *int) string {
	if rpm == nil {
		return ""
	}
	return fmt.Sprintf("Fan %d rpm", *rpm)
}

// decorateCPUFan places the fan reading after the CPU progress bar. Regular
// layout has no right box border. Landscape first consumes box padding and, if
// needed, shortens only the CPU bar so the two-column geometry never moves.
func decorateCPUFan(frame string, w int, rpm *int) string {
	text := fanRPMText(rpm)
	if text == "" {
		return frame
	}
	visibleAdded := 1 + utf8.RuneCountInString(text)
	styled := " " + white + "Fan" + reset + " " + lightGray + fmt.Sprintf("%d rpm", *rpm) + reset

	lines := strings.SplitAfter(frame, "\n")
	for i, line := range lines {
		plain := plainTerminalLine(line)
		if !strings.Contains(plain, "CPU") || !strings.Contains(plain, "]") {
			continue
		}

		if strings.Count(plain, "│") >= 2 {
			original := line
			rightEdge := nthRuneByteIndex(line, '│', 2)
			closeBar := strings.Index(line, "]")
			if rightEdge <= closeBar || closeBar < 0 {
				continue
			}

			start := rightEdge
			padding := 0
			for start > closeBar+1 && padding < visibleAdded && line[start-1] == ' ' {
				start--
				padding++
			}
			line = line[:start] + line[rightEdge:]
			remaining := visibleAdded - padding

			for remaining > 0 {
				closeBar = strings.Index(line, "]")
				if closeBar <= 0 {
					line = original
					break
				}
				rn, size := utf8.DecodeLastRuneInString(line[:closeBar])
				if size <= 0 || rn == '[' {
					line = original
					break
				}
				line = line[:closeBar-size] + line[closeBar:]
				remaining--
			}
			if line == original {
				continue
			}
			closeBar = strings.Index(line, "]")
			line = line[:closeBar+1] + styled + line[closeBar+1:]
		} else {
			if utf8.RuneCountInString(plain)+visibleAdded > w {
				continue
			}
			closeBar := strings.Index(line, "]")
			if closeBar < 0 {
				continue
			}
			line = line[:closeBar+1] + styled + line[closeBar+1:]
		}

		lines[i] = line
		break
	}
	return strings.Join(lines, "")
}
