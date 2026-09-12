package tui

import "fmt"

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

// fitProgressLine builds the bar at its final width instead of resizing an
// already rendered terminal row. targetWidth is the desired visible width of
// the whole line/content. The suffix is expected to include its own leading
// spacing and ANSI styling.
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
