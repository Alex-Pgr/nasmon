package tui

import (
	"fmt"
	"strings"

	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

// buildSystemRows is the single source of truth for System section content.
// Regular rows include their left section border; landscape rows are box
// content and let makeBox add borders later.
func buildSystemRows(s model.Snapshot, width int, landscape bool) []string {
	if landscape {
		traffic := "measuring..."
		if s.NetReady {
			traffic = fmt.Sprintf("RX %s TX %s", speed(s.RXBps), speed(s.TXBps))
		}
		dio := "measuring..."
		if s.DiskIOReady {
			dio = fmt.Sprintf("R %s W %s", speed(s.DiskReadBps), speed(s.DiskWriteBps))
		}
		netLabel := "No IP"
		if s.IP != "" {
			netLabel = s.IP
		}

		rows := []string{fmt.Sprintf(" %sUptime%s %s%s%s", white, reset, lightGray, dur(s.Uptime), reset)}
		rows = append(rows, systemMetricRows(s, width, true)...)
		rows = append(rows,
			fmt.Sprintf(" %sSwap%s %s%d%%%s %s%s/%sG%s  %sIOwait%s %s%d%%%s", white, reset, pctColor(s.SwapPercent, "mem"), s.SwapPercent, reset, gray, gib(s.SwapUsedBytes), gib(s.SwapTotalBytes), reset, white, reset, pctColor(s.IOWait, "io"), s.IOWait, reset),
			fmt.Sprintf(" %sLoad%s %s%s%s %s(%s, %s)%s", white, reset, lightGray, s.Load1, reset, gray, s.Load5, s.Load15, reset),
			fmt.Sprintf(" %sNet%s  %s%s%s", white, reset, blue, netLabel, reset),
			fmt.Sprintf(" %sTraffic%s %s%s%s", white, reset, gray, traffic, reset),
			fmt.Sprintf(" %sDisk%s %s%s%s", white, reset, gray, dio, reset),
		)
		return rows
	}

	rows := []string{fmt.Sprintf("%s│ Uptime:%s %s%s%s", white, reset, lightGray, dur(s.Uptime), reset)}
	rows = append(rows, systemMetricRows(s, width, false)...)
	rows = append(rows, fmt.Sprintf("%s│ Swap:%s %s%d%%%s %s%s/%s GiB%s  %sIOwait:%s %s%d%%%s", white, reset, pctColor(s.SwapPercent, "mem"), s.SwapPercent, reset, gray, gib(s.SwapUsedBytes), gib(s.SwapTotalBytes), reset, white, reset, pctColor(s.IOWait, "io"), s.IOWait, reset))

	netLabel := "No IP"
	if s.IP != "" {
		netLabel = fmt.Sprintf("Ethernet (%s)", s.IP)
	}
	rows = append(rows, fmt.Sprintf("%s│ Net:%s %s%s%s", white, reset, blue, netLabel, reset))

	traffic := "Измеряем..."
	if s.NetReady {
		traffic = fmt.Sprintf("RX %s  TX %s", speed(s.RXBps), speed(s.TXBps))
	}
	rows = append(rows, fmt.Sprintf("%s│ Traffic:%s %s%s%s", white, reset, gray, traffic, reset))

	dio := "Измеряем..."
	if s.DiskIOReady {
		dio = fmt.Sprintf("R %s  W %s", speed(s.DiskReadBps), speed(s.DiskWriteBps))
	}
	rows = append(rows, fmt.Sprintf("%s│ Disk IO:%s %s%s%s", white, reset, gray, dio, reset))

	fu := green + "0" + reset
	if s.FailedUnits > 0 {
		fu = yellow + fmt.Sprint(s.FailedUnits) + reset
	}
	rows = append(rows, fmt.Sprintf("%s│ Load:%s %s%s%s %s(%s, %s)%s  %sFailed units:%s %s", white, reset, lightGray, s.Load1, reset, gray, s.Load5, s.Load15, reset, white, reset, fu))
	return rows
}

func dockerRowState(c model.Container) (icon, color string) {
	icon, color = "●", blue
	if c.State != "running" || c.Health == "unhealthy" {
		return "⚠", yellow
	}
	if c.Health == "healthy" {
		return "✓", green
	}
	return icon, color
}

// buildDockerRows owns Docker column sizing and row formatting for both layouts.
func buildDockerRows(containers []model.Container, width int, landscape bool, mode DockerSortMode) []string {
	const ramW = 6
	namesLabel, ramLabel := dockerSortLabels(mode)

	if landscape {
		nameW := clamp((width+2)/3, 10, 24)
		statusW := width - nameW - ramW - 7
		if statusW < 6 {
			nameW -= 6 - statusW
			if nameW < 8 {
				nameW = 8
			}
			statusW = width - nameW - ramW - 7
		}
		if statusW < 1 {
			statusW = 1
		}

		rows := []string{fmt.Sprintf("   %-*s  %*s  STATUS", nameW, namesLabel, ramW, ramLabel)}
		for _, c := range containers {
			icon, color := dockerRowState(c)
			status := dockerStatus(c)
			if status == "" {
				status = c.State
			}
			status += fmt.Sprintf(" R:%d", c.Restarts)
			rows = append(rows, fmt.Sprintf(" %s%s%s %s%s%s  %s%s%s  %s%s%s", color, icon, reset, lightGray, padRightCells(c.Name, nameW), reset, gray, padLeftCells(dockerMemoryLabel(c.MemoryBytes), ramW), reset, color, trunc(status, statusW), reset))
		}
		return rows
	}

	nameW := clamp(width/3, 10, 24)
	statusW := width - 8 - nameW - ramW
	if statusW < 6 {
		statusW = 6
	}
	rows := []string{fmt.Sprintf("%s│   %-*s  %*s  STATUS%s", white, nameW, namesLabel, ramW, ramLabel, reset)}
	for _, c := range containers {
		icon, color := dockerRowState(c)
		status := dockerStatus(c)
		if status == "" {
			status = c.State
		}
		status += fmt.Sprintf(" R:%d", c.Restarts)
		rows = append(rows, fmt.Sprintf("%s│ %s%s%s %s%s%s  %s%s%s  %s%s%s", white, color, icon, reset, lightGray, padRightCells(c.Name, nameW), reset, gray, padLeftCells(dockerMemoryLabel(c.MemoryBytes), ramW), reset, color, trunc(status, statusW), reset))
	}
	return rows
}

func buildDiskRows(usage []model.DiskUsage, landscape bool) []string {
	if landscape {
		pathW, usedW, totalW := landscapeDiskWidths(usage)
		rows := make([]string, 0, len(usage))
		for _, d := range usage {
			rows = append(rows, fmt.Sprintf(" %s%s%s  %s%s/%s%s  %s%3d%%%s", lightGray, padRightCells(d.Path, pathW), reset, white, padLeftCells(humanBytes(d.UsedBytes), usedW), padRightCells(humanBytes(d.TotalBytes), totalW), reset, pctColor(d.Percent, "disk"), d.Percent, reset))
		}
		return rows
	}

	rows := make([]string, 0, len(usage))
	for _, d := range usage {
		rows = append(rows, fmt.Sprintf("%s│ %s%s%s %7s/%-7s %s%3d%%%s", white, lightGray, padRightCells(d.Path, 14), reset, humanBytes(d.UsedBytes), humanBytes(d.TotalBytes), pctColor(d.Percent, "disk"), d.Percent, reset))
	}
	return rows
}

func buildHealthRows(s model.Snapshot, landscape bool) []string {
	labels := diskHealthLabels(s.DiskHealth)
	deviceW := healthDeviceWidth(s.DiskHealth)
	rows := make([]string, 0, len(s.DiskHealth)+1)
	if landscape {
		if s.FailedUnits == 0 {
			rows = append(rows, fmt.Sprintf(" %ssystemd OK (0 failed)%s", green, reset))
		} else {
			rows = append(rows, fmt.Sprintf(" %ssystemd WARNING (%d failed)%s", yellow, s.FailedUnits, reset))
		}
	}

	for i, h := range s.DiskHealth {
		label := labels[i]
		mark := "SMART " + h.Health
		markColor := yellow
		if diskHealthOK(h) {
			mark = "SMART OK"
			markColor = green
		}
		sleepMark := ""
		if h.Sleeping {
			sleepMark = " " + blue + "SLEEP" + reset
		}
		if landscape {
			rows = append(rows, fmt.Sprintf(" %s%s%s %s%-5s%s %s%s%s %s%s%s%s", lightGray, padRightCells(strings.TrimSpace(label), deviceW), reset, white, strings.TrimSpace(h.Temperature), reset, markColor, mark, reset, gray, diskHealthDetails(h, true), reset, sleepMark))
			continue
		}
		rows = append(rows, fmt.Sprintf("%s│ %s%s%s  %-5s %s%s%s %s%s%s%s", white, lightGray, padRightCells(label, deviceW), reset, h.Temperature, markColor, mark, reset, gray, diskHealthDetails(h, false), reset, sleepMark))
	}
	return rows
}
