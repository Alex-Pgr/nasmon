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

func shortenMetricBar(line string, cells int) string {
	if cells <= 0 {
		return line
	}
	closeBar := strings.Index(line, "]")
	if closeBar < 0 {
		return line
	}
	removed := 0
	for removed < cells {
		rn, size := utf8.DecodeLastRuneInString(line[:closeBar])
		if size <= 0 || rn == '[' {
			break
		}
		line = line[:closeBar-size] + line[closeBar:]
		closeBar -= size
		removed++
	}
	if removed > 0 {
		line = line[:closeBar+1] + strings.Repeat(" ", removed) + line[closeBar+1:]
	}
	return line
}

// CPU and RAM bars are intentionally two cells shorter than the other bars.
// Keep the total terminal row width unchanged so landscape box borders stay
// aligned. ZRAM is intentionally left at its original width.
func shortenCPURAMBars(frame string) string {
	lines := strings.SplitAfter(frame, "\n")
	for i, line := range lines {
		plain := plainTerminalLine(line)
		isCPU := strings.Contains(plain, "CPU")
		isRAM := strings.Contains(plain, "RAM") && !strings.Contains(plain, "ZRAM")
		if (isCPU || isRAM) && strings.Contains(plain, "[") && strings.Contains(plain, "]") {
			lines[i] = shortenMetricBar(line, 2)
		}
	}
	return strings.Join(lines, "")
}

// decorateCPUFan places the fan reading after the CPU progress bar. Regular
// layout has no right box border. Landscape first consumes box padding and, if
// needed, shortens only the CPU bar so the two-column geometry never moves.
func decorateCPUFan(frame string, w int, rpm *int) string {
	frame = shortenCPURAMBars(frame)
	text := fanRPMText(rpm)
	if text == "" {
		return frame
	}
	visibleAdded := 1 + utf8.RuneCountInString(text)
	styled := " " + gray + text + reset

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
