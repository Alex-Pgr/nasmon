package tui

import (
	"fmt"
	"strings"

	"nasmon/internal/model"
)

func fanRPMText(rpm *int) string {
	if rpm == nil {
		return ""
	}
	return fmt.Sprintf("Fan %d rpm", *rpm)
}

func metricBar(p, w int) string {
	p = clamp(p, 0, 100)
	if w < 1 {
		w = 1
	}
	filled := p * w / 100
	return rep("█", filled) + rep("░", w-filled)
}

func nthRuneIndex(s string, target rune, want int) int {
	seen := 0
	idx := 0
	for _, r := range s {
		if r == target {
			seen++
			if seen == want {
				return idx
			}
		}
		idx++
	}
	return -1
}

// fitProgressLine builds the bar at its final width. targetWidth is the desired
// visible width of the whole line/content. suffix includes its leading spacing
// and ANSI styling.
func fitProgressLine(prefix string, percent int, suffix string, targetWidth int) string {
	barW := targetWidth - visibleRunes(prefix) - 2 - visibleRunes(suffix) // '[' + ']'
	if barW < 1 {
		barW = 1
	}
	return prefix + "[" + metricBar(percent, barW) + "]" + reset + suffix
}

func fanSuffix(rpm *int) string {
	text := fanRPMText(rpm)
	if text == "" {
		return ""
	}
	return " " + gray + text + reset
}

// rewriteRegularProgressLine is safe only for the single-column regular
// layout: it keeps the original styled prefix up to '[' and rebuilds everything
// to the right. Unlike the previous implementation it never touches landscape
// rows that also contain the Docker box.
func rewriteRegularProgressLine(line string, width, percent int, suffix string) string {
	open := strings.Index(line, "[")
	if open < 0 {
		return line
	}
	newline := ""
	if strings.HasSuffix(line, "\n") {
		newline = "\n"
	}
	prefix := line[:open]
	return fitProgressLine(prefix, percent, suffix, width) + newline
}

func decorateRegularSystemBars(frame string, width int, s model.Snapshot) string {
	lines := strings.SplitAfter(frame, "\n")
	for i, line := range lines {
		plain := plainTerminalLine(line)
		if !strings.Contains(plain, "[") || !strings.Contains(plain, "]") {
			continue
		}
		switch {
		case strings.Contains(plain, "ZRAM:"):
			tail := " " + gray + fmt.Sprintf("%s/%s GiB", gib(s.ZRAMUsedBytes), gib(s.ZRAMTotalBytes)) + reset
			lines[i] = rewriteRegularProgressLine(line, width, s.ZRAMPercent, tail)
		case strings.Contains(plain, "RAM:"):
			tail := " " + gray + fmt.Sprintf("%s/%s GiB", gib(s.MemUsedBytes), gib(s.MemTotalBytes)) + reset
			lines[i] = rewriteRegularProgressLine(line, width, s.MemPercent, tail)
		case strings.Contains(plain, "CPU:"):
			lines[i] = rewriteRegularProgressLine(line, width, s.CPUUsage, fanSuffix(s.FanRPM))
		}
	}
	return strings.Join(lines, "")
}
