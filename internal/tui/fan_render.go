package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

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

func metricSegment(plain string, landscape bool) string {
	if !landscape {
		return plain
	}
	if edge := nthRuneIndex(plain, '│', 2); edge >= 0 {
		runes := []rune(plain)
		return string(runes[:edge])
	}
	return plain
}

func rewriteProgressLine(line string, screenW int, landscape bool, percent int, suffix string) string {
	openRaw := strings.Index(line, "[")
	if openRaw < 0 {
		return line
	}
	plain := plainTerminalLine(line)
	openCol := runeIndex(plain, "[")
	if openCol < 0 {
		return line
	}

	suffixCells := 0
	styledSuffix := ""
	if suffix != "" {
		suffixCells = 1 + utf8.RuneCountInString(suffix)
		styledSuffix = " " + gray + suffix + reset
	}

	barW := 1
	if landscape {
		borderCol := nthRuneIndex(plain, '│', 2)
		rightEdge := nthRuneByteIndex(line, '│', 2)
		if borderCol < 0 || rightEdge < 0 {
			return line
		}
		// prefix + '[' + bar + ']' + suffix + one blank + right border
		barW = borderCol - openCol - suffixCells - 3
		if barW < 1 {
			barW = 1
		}
		return line[:openRaw+1] + metricBar(percent, barW) + "]" + reset + styledSuffix + white + " " + line[rightEdge:]
	}

	// The suffix should end one terminal cell before the physical right edge.
	targetLen := screenW - 1
	barW = targetLen - openCol - suffixCells - 2
	if barW < 1 {
		barW = 1
	}
	newline := ""
	if strings.HasSuffix(line, "\n") {
		newline = "\n"
	}
	return line[:openRaw+1] + metricBar(percent, barW) + "]" + reset + styledSuffix + reset + newline
}

// decorateSystemBars makes CPU, RAM and ZRAM bars consume all available width.
// Their trailing labels finish one cell before the terminal edge in regular
// layout, or one cell before the System box's right border in landscape.
func decorateSystemBars(frame string, screenW int, landscape bool, s model.Snapshot) string {
	lines := strings.SplitAfter(frame, "\n")
	for i, line := range lines {
		plain := plainTerminalLine(line)
		segment := metricSegment(plain, landscape)
		if !strings.Contains(segment, "[") || !strings.Contains(segment, "]") {
			continue
		}

		switch {
		case strings.Contains(segment, "ZRAM"):
			tail := fmt.Sprintf("%s/%s GiB", gib(s.ZRAMUsedBytes), gib(s.ZRAMTotalBytes))
			if landscape {
				tail = fmt.Sprintf("%s/%sG", gib(s.ZRAMUsedBytes), gib(s.ZRAMTotalBytes))
			}
			lines[i] = rewriteProgressLine(line, screenW, landscape, s.ZRAMPercent, tail)
		case strings.Contains(segment, "RAM"):
			tail := fmt.Sprintf("%s/%s GiB", gib(s.MemUsedBytes), gib(s.MemTotalBytes))
			if landscape {
				tail = fmt.Sprintf("%s/%sG", gib(s.MemUsedBytes), gib(s.MemTotalBytes))
			}
			lines[i] = rewriteProgressLine(line, screenW, landscape, s.MemPercent, tail)
		case strings.Contains(segment, "CPU"):
			lines[i] = rewriteProgressLine(line, screenW, landscape, s.CPUUsage, fanRPMText(s.FanRPM))
		}
	}
	return strings.Join(lines, "")
}
