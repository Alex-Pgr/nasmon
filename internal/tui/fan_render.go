package tui

import (
	"fmt"

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

// padANSI left-aligns a styled value inside a fixed-width column. The column
// always starts exactly one cell after the progress bar, so all metric suffixes
// begin at the same position while the overall row width remains stable.
func padANSI(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if visibleRunes(s) > w {
		return truncANSI(s, w)
	}
	return s + rep(" ", w-visibleRunes(s))
}

func gpuMetric(usage *int) (int, string, string) {
	if usage == nil {
		return 0, "N/A", gray
	}
	value := clamp(*usage, 0, 100)
	color := gray
	switch {
	case value >= 90:
		color = orange
	case value >= 60:
		color = yellow
	case value >= 10:
		color = green
	}
	return value, fmt.Sprintf("%d%%", value), color
}

func vcnMetricSuffix(vcn string) string {
	if vcn == "" {
		vcn = "N/A"
	}
	color := gray
	if vcn == "ACTIVE" {
		color = green
	}
	return white + "VCN " + reset + color + vcn + reset
}

func systemMetricRows(s model.Snapshot, targetWidth int, landscape bool) []string {
	lead := white + "│ " + reset
	labelW := 5
	suffixW := 16
	labels := []string{"CPU:", "GPU:", "RAM:", "ZRAM:"}
	if landscape {
		lead = " "
		labelW = 4
		suffixW = 14
		labels = []string{"CPU", "GPU", "RAM", "ZRAM"}
	}

	// All rows share the same fixed prefix and suffix-column width. On a very
	// narrow terminal shrink the suffix column for every row together, never
	// independently, so the progress bars remain aligned.
	prefixCells := visibleRunes(lead) + labelW + 1 + 5 + 2 + 4
	maxSuffix := targetWidth - prefixCells - 2 - 1 - 4 // brackets, gap, min bar
	if suffixW > maxSuffix {
		suffixW = maxSuffix
	}
	if suffixW < 6 {
		suffixW = 6
	}

	makeRow := func(label, tempText, tempStyle, pctText, pctStyle string, percent int, suffix string) string {
		prefix := lead + white + fmt.Sprintf("%-*s", labelW, label) + reset + " " +
			tempStyle + fmt.Sprintf("%-5s", tempText) + reset + "  " +
			pctStyle + fmt.Sprintf("%-4s", pctText) + reset + gray
		barW := targetWidth - visibleRunes(prefix) - 2 - 1 - suffixW
		if barW < 1 {
			barW = 1
		}
		return prefix + "[" + metricBar(percent, barW) + "]" + reset + " " + padANSI(suffix, suffixW)
	}

	gpuValue, gpuText, gpuColor := gpuMetric(s.GPUUsage)
	ramSuffix := gray + fmt.Sprintf("%s/%s GiB", gib(s.MemUsedBytes), gib(s.MemTotalBytes)) + reset
	zramSuffix := gray + fmt.Sprintf("%s/%s GiB", gib(s.ZRAMUsedBytes), gib(s.ZRAMTotalBytes)) + reset
	if landscape {
		ramSuffix = gray + fmt.Sprintf("%s/%sG", gib(s.MemUsedBytes), gib(s.MemTotalBytes)) + reset
		zramSuffix = gray + fmt.Sprintf("%s/%sG", gib(s.ZRAMUsedBytes), gib(s.ZRAMTotalBytes)) + reset
	}

	rows := []string{
		makeRow(labels[0], temp(s.CPUTempC), tempColor(s.CPUTempC), fmt.Sprintf("%d%%", s.CPUUsage), pctColor(s.CPUUsage, "cpu"), s.CPUUsage, gray+fanRPMText(s.FanRPM)+reset),
		makeRow(labels[1], temp(s.GPUTempC), lightGray, gpuText, gpuColor, gpuValue, vcnMetricSuffix(s.GPUVCN)),
		makeRow(labels[2], "", reset, fmt.Sprintf("%d%%", s.MemPercent), pctColor(s.MemPercent, "mem"), s.MemPercent, ramSuffix),
	}
	if s.ZRAMTotalBytes > 0 {
		rows = append(rows, makeRow(labels[3], "", reset, fmt.Sprintf("%d%%", s.ZRAMPercent), pctColor(s.ZRAMPercent, "mem"), s.ZRAMPercent, zramSuffix))
	}
	return rows
}
