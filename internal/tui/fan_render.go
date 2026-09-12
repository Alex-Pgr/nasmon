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
	return fmt.Sprintf("Fan %drpm", *rpm)
}

// decorateCPUFan places the fan reading after the CPU progress bar. Regular
// layout has no right box border; landscape consumes existing padding before
// the left box's closing border so the two-column geometry stays unchanged.
func decorateCPUFan(frame string, w int, rpm *int) string {
	text := fanRPMText(rpm)
	if text == "" {
		return frame
	}
	visibleAdded := 1 + utf8.RuneCountInString(text)
	styled := " " + white + "Fan" + reset + " " + lightGray + fmt.Sprintf("%drpm", *rpm) + reset

	lines := strings.SplitAfter(frame, "\n")
	for i, line := range lines {
		plain := plainTerminalLine(line)
		if !strings.Contains(plain, "CPU") || !strings.Contains(plain, "]") {
			continue
		}

		closeBar := strings.Index(line, "]")
		if closeBar < 0 {
			continue
		}
		insertAt := closeBar + 1

		if strings.Count(plain, "│") >= 2 {
			rightEdge := nthRuneByteIndex(line, '│', 2)
			if rightEdge <= insertAt {
				continue
			}
			start := rightEdge
			removed := 0
			for start > insertAt && removed < visibleAdded && line[start-1] == ' ' {
				start--
				removed++
			}
			if removed != visibleAdded {
				continue
			}
			line = line[:start] + line[rightEdge:]
		} else if utf8.RuneCountInString(plain)+visibleAdded > w {
			continue
		}

		line = line[:insertAt] + styled + line[insertAt:]
		lines[i] = line
		break
	}
	return strings.Join(lines, "")
}
