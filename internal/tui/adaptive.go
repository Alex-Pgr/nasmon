package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"nasmon/internal/model"
)

func dockerMemoryLabel(b uint64) string {
	if b == 0 {
		return "-"
	}
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0fM", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0fK", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func dockerStatus(c model.Container) string {
	txt := c.Status
	if c.Health != "" {
		txt = strings.ReplaceAll(txt, " ("+c.Health+")", "")
		txt = strings.ReplaceAll(txt, " (health: "+c.Health+")", "")
	}
	txt = strings.NewReplacer(
		" seconds", "s",
		" second", "s",
		" minutes", "m",
		" minute", "m",
		" hours", "h",
		" hour", "h",
	).Replace(txt)
	return strings.TrimSpace(txt)
}

func healthDeviceWidth(health []model.DiskHealth) int {
	width := 3
	for _, h := range health {
		if n := utf8.RuneCountInString(h.Device); n > width {
			width = n
		}
	}
	return clamp(width, 3, 16)
}

func gpuUsageLabel(usage *int) string {
	if usage == nil {
		return "N/A"
	}
	return fmt.Sprintf("%d%%", *usage)
}

func selectDockerContainers(containers []model.Container, limit int) []model.Container {
	if limit <= 0 {
		return nil
	}
	if len(containers) <= limit {
		return append([]model.Container(nil), containers...)
	}

	restarted := make([]model.Container, 0)
	others := make([]model.Container, 0)
	for _, c := range containers {
		if c.Restarts > 0 {
			restarted = append(restarted, c)
		} else {
			others = append(others, c)
		}
	}

	sort.SliceStable(restarted, func(i, j int) bool {
		if restarted[i].Restarts != restarted[j].Restarts {
			return restarted[i].Restarts > restarted[j].Restarts
		}
		if restarted[i].MemoryBytes != restarted[j].MemoryBytes {
			return restarted[i].MemoryBytes > restarted[j].MemoryBytes
		}
		return strings.ToLower(restarted[i].Name) < strings.ToLower(restarted[j].Name)
	})
	sort.SliceStable(others, func(i, j int) bool {
		if others[i].MemoryBytes != others[j].MemoryBytes {
			return others[i].MemoryBytes > others[j].MemoryBytes
		}
		return strings.ToLower(others[i].Name) < strings.ToLower(others[j].Name)
	})

	out := make([]model.Container, 0, limit)
	for _, list := range [][]model.Container{restarted, others} {
		for _, c := range list {
			if len(out) == limit {
				return out
			}
			out = append(out, c)
		}
	}
	return out
}

func regularSystemRows(s model.Snapshot) int {
	rows := 11
	if s.ZRAMTotalBytes > 0 {
		rows++
	}
	return rows
}

// regularFixedRows includes all regular-layout rows except Docker container
// data rows and the optional two-line run footer.
func regularFixedRows(s model.Snapshot, headerRows int) int {
	return headerRows + regularSystemRows(s) + len(s.DiskUsage) + len(s.DiskHealth) + 15
}

func (r Renderer) RenderAdaptive(s model.Snapshot, rows, cols int) string {
	if r.Config.ForceCols > 0 && r.Config.ForceRows > 0 {
		cols = r.Config.ForceCols
		rows = r.Config.ForceRows
	}
	if cols < r.Config.MinTermWidth {
		cols = r.Config.MinTermWidth
	}
	draw := cols - r.Config.RightMargin
	if draw < 32 {
		draw = 32
	}
	if rows <= 30 && cols >= 80 {
		return r.renderLandscapeAdaptive(s, draw, rows)
	}
	return r.renderRegularAdaptive(s, draw, rows)
}

func (r Renderer) renderRegularAdaptive(s model.Snapshot, w, rows int) string {
	fullRows := regularFixedRows(s, 5) + len(s.Containers)
	compactHeader := rows > 0 && fullRows > rows

	containers := append([]model.Container(nil), s.Containers...)
	if compactHeader && rows > 0 {
		available := rows - regularFixedRows(s, 1)
		if available < len(containers) {
			containers = selectDockerContainers(containers, available)
		}
	}

	var b strings.Builder
	n := 0
	addn := func(x string) { add(&b, x); n++ }
	if compactHeader {
		addn(cyan + center(fmt.Sprintf("NAS Health Monitor %s", r.Config.MainInterval), w) + reset)
	} else {
		info := fmt.Sprintf("%s • Обновление: %s", s.UpdatedAt.Format("2006-01-02 15:04:05"), r.Config.MainInterval)
		addn(cyan + full(w) + reset)
		addn(white + center("NAS Health Monitor", w) + reset)
		addn(gray + center(trunc(info, w), w) + reset)
		addn(cyan + full(w) + reset)
		addn("")
	}

	addn(white + "┌── System Status" + reset)
	addn(fmt.Sprintf("%s│ Uptime:%s %s%s%s", white, reset, lightGray, dur(s.Uptime), reset))
	ramTail := fmt.Sprintf("%s/%s GiB", gib(s.MemUsedBytes), gib(s.MemTotalBytes))
	bw := clamp(w-(6+1+5+2+4+1+2+utf8.RuneCountInString(ramTail)), 10, 26)
	addn(fmt.Sprintf("%s│ CPU:%s %s%-5s%s  %s%-4s%s%s[%s]%s", white, reset, tempColor(s.CPUTempC), temp(s.CPUTempC), reset, pctColor(s.CPUUsage, "cpu"), fmt.Sprintf("%d%%", s.CPUUsage), reset, gray, bar(s.CPUUsage, bw), reset))
	vcnColor := gray
	if s.GPUVCN == "ACTIVE" {
		vcnColor = green
	}
	addn(fmt.Sprintf("%s│ GPU:%s %s%-5s%s  %s%-4s%s %sVCN:%s %s%-6s%s", white, reset, lightGray, temp(s.GPUTempC), reset, lightGray, gpuUsageLabel(s.GPUUsage), reset, white, reset, vcnColor, s.GPUVCN, reset))
	addn(fmt.Sprintf("%s│ RAM:%s        %s%-4s%s%s[%s] %s%s", white, reset, pctColor(s.MemPercent, "mem"), fmt.Sprintf("%d%%", s.MemPercent), reset, gray, bar(s.MemPercent, bw), ramTail, reset))
	if s.ZRAMTotalBytes > 0 {
		zramTail := fmt.Sprintf("%s/%s GiB", gib(s.ZRAMUsedBytes), gib(s.ZRAMTotalBytes))
		addn(fmt.Sprintf("%s│ ZRAM:%s       %s%-4s%s%s[%s] %s%s", white, reset, pctColor(s.ZRAMPercent, "mem"), fmt.Sprintf("%d%%", s.ZRAMPercent), reset, gray, bar(s.ZRAMPercent, bw), zramTail, reset))
	}
	addn(fmt.Sprintf("%s│ Swap:%s %s%d%%%s %s%s/%s GiB%s  %sIOwait:%s %s%d%%%s", white, reset, pctColor(s.SwapPercent, "mem"), s.SwapPercent, reset, gray, gib(s.SwapUsedBytes), gib(s.SwapTotalBytes), reset, white, reset, pctColor(s.IOWait, "io"), s.IOWait, reset))
	netLabel := "No IP"
	if s.IP != "" {
		netLabel = fmt.Sprintf("Ethernet (%s)", s.IP)
	}
	addn(fmt.Sprintf("%s│ Net:%s %s%s%s", white, reset, blue, netLabel, reset))
	traffic := "Измеряем..."
	if s.NetReady {
		traffic = fmt.Sprintf("RX %s  TX %s", speed(s.RXBps), speed(s.TXBps))
	}
	addn(fmt.Sprintf("%s│ Traffic:%s %s%s%s", white, reset, gray, traffic, reset))
	dio := "Измеряем..."
	if s.DiskIOReady {
		dio = fmt.Sprintf("R %s  W %s", speed(s.DiskReadBps), speed(s.DiskWriteBps))
	}
	addn(fmt.Sprintf("%s│ Disk IO:%s %s%s%s", white, reset, gray, dio, reset))
	fu := green + "0" + reset
	if s.FailedUnits > 0 {
		fu = yellow + fmt.Sprint(s.FailedUnits) + reset
	}
	addn(fmt.Sprintf("%s│ Load:%s %s%s%s %s(%s, %s)%s  %sFailed units:%s %s", white, reset, lightGray, s.Load1, reset, gray, s.Load5, s.Load15, reset, white, reset, fu))
	addn(white + bottom(w) + reset)

	addn("")
	addn(white + "┌── Disk Usage" + reset)
	for _, d := range s.DiskUsage {
		addn(fmt.Sprintf("%s│ %s%-14s%s %7s/%-7s %s%3d%%%s", white, lightGray, trunc(d.Path, 14), reset, humanBytes(d.UsedBytes), humanBytes(d.TotalBytes), pctColor(d.Percent, "disk"), d.Percent, reset))
	}
	addn(white + bottom(w) + reset)

	addn("")
	addn(white + "┌── Health" + reset)
	deviceW := healthDeviceWidth(s.DiskHealth)
	for _, h := range s.DiskHealth {
		mark := yellow + "SMART " + h.Health + reset
		if diskHealthOK(h) {
			mark = green + "✓ SMART OK" + reset
		}
		sleepMark := ""
		if h.Sleeping {
			sleepMark = " " + blue + "SLEEP" + reset
		}
		addn(fmt.Sprintf("%s│ %s%-*s%s  %-5s %s %s%s%s%s", white, lightGray, deviceW, trunc(h.Device, deviceW), reset, h.Temperature, mark, gray, diskHealthDetails(h, false), reset, sleepMark))
	}
	addn(white + bottom(w) + reset)

	addn("")
	addn(white + "┌── Docker Services" + reset)
	const ramW = 6
	nameW := clamp(w/3, 10, 24)
	statusW := w - 8 - nameW - ramW
	if statusW < 6 {
		statusW = 6
	}
	addn(fmt.Sprintf("%s│   %-*s  %*s  STATUS%s", white, nameW, "NAMES", ramW, "RAM", reset))
	for _, c := range containers {
		icon, color := "●", blue
		if c.State != "running" || c.Health == "unhealthy" {
			icon, color = "⚠", yellow
		} else if c.Health == "healthy" {
			icon, color = "✓", green
		}
		txt := dockerStatus(c)
		if txt == "" {
			txt = c.State
		}
		txt += fmt.Sprintf(" R:%d", c.Restarts)
		addn(fmt.Sprintf("%s│ %s%s%s %s%-*s%s  %s%*s%s  %s%s%s", white, color, icon, reset, lightGray, nameW, trunc(c.Name, nameW), reset, gray, ramW, dockerMemoryLabel(c.MemoryBytes), reset, color, trunc(txt, statusW), reset))
	}
	addn(white + bottom(w) + reset)

	addn("")
	addn(white + "┌── Storage Analysis" + reset)
	addn(fmt.Sprintf("%s│ %s%s:%s %s%s/%s GiB%s", white, lightGray, s.StoragePath, reset, pctColor(s.StoragePercent, "disk"), gib(s.StorageUsedBytes), gib(s.StorageTotalBytes), reset))
	sbw := clamp(w/2, 20, 60)
	addn(fmt.Sprintf("%s│ %s[%s]%s %s%d%%%s", white, gray, bar(s.StoragePercent, sbw), reset, pctColor(s.StoragePercent, "disk"), s.StoragePercent, reset))
	addn(white + bottom(w) + reset)
	if n+2 < rows {
		addn("")
		addn(gray + center(fmt.Sprintf("── run %s ──", dur(time.Since(s.StartedAt))), w) + reset)
	}
	return b.String()
}

func (r Renderer) renderLandscapeAdaptive(s model.Snapshot, w, rows int) string {
	lowerRows := len(s.DiskUsage)
	if h := 1 + len(s.DiskHealth); h > lowerRows {
		lowerRows = h
	}
	upperRows := 7
	if len(s.Containers) > upperRows {
		upperRows = len(s.Containers)
	}
	fullRows := 8 + upperRows + lowerRows
	compactHeader := rows > 0 && fullRows > rows

	copySnap := s
	copySnap.Containers = append([]model.Container(nil), s.Containers...)
	copySnap.DiskHealth = append([]model.DiskHealth(nil), s.DiskHealth...)
	deviceW := healthDeviceWidth(copySnap.DiskHealth)
	for i := range copySnap.DiskHealth {
		copySnap.DiskHealth[i].Device = fmt.Sprintf("%-*s", deviceW, trunc(copySnap.DiskHealth[i].Device, deviceW))
		copySnap.DiskHealth[i].Temperature = fmt.Sprintf("%-5s", copySnap.DiskHealth[i].Temperature)
	}
	if compactHeader && rows > 0 {
		availableUpper := rows - 6 - lowerRows
		if availableUpper < len(copySnap.Containers) {
			copySnap.Containers = selectDockerContainers(copySnap.Containers, availableUpper)
		}
	}
	for i := range copySnap.Containers {
		status := dockerStatus(copySnap.Containers[i])
		if status == "" {
			status = copySnap.Containers[i].State
		}
		copySnap.Containers[i].Status = fmt.Sprintf("%s RAM:%s", status, dockerMemoryLabel(copySnap.Containers[i].MemoryBytes))
	}

	out := r.renderLandscape(copySnap, w)
	out = strings.Replace(out, "VCN", fmt.Sprintf("%-4s VCN", gpuUsageLabel(copySnap.GPUUsage)), 1)
	out = strings.ReplaceAll(out, " (healthy)", "")
	out = strings.ReplaceAll(out, " (unhealthy)", "")
	out = strings.ReplaceAll(out, " (health: starting)", "")
	if !compactHeader {
		return out
	}

	parts := strings.SplitAfter(out, "\n")
	if len(parts) < 3 {
		return out
	}
	var head strings.Builder
	add(&head, cyan+center(fmt.Sprintf("NAS Health Monitor %s", r.Config.MainInterval), w)+reset)
	return head.String() + strings.Join(parts[3:], "")
}
