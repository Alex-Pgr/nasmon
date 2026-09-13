package tui

import (
	"fmt"
	"strings"
	"time"

	"nasmon/internal/model"
)

// renderRegularInteractiveAdaptive is the canonical regular runtime layout.
// It selects/sorts Docker rows before rendering and composes already-built
// section rows directly; no rendered ANSI frame is parsed or rewritten.
func (r Renderer) renderRegularInteractiveAdaptive(s model.Snapshot, w, rows int, mode DockerSortMode) string {
	fullRows := regularFixedRows(s, 5) + len(s.Containers)
	compactHeader := rows > 0 && fullRows > rows

	containers := append([]model.Container(nil), s.Containers...)
	if compactHeader && rows > 0 {
		available := rows - regularFixedRows(s, 1)
		if available < len(containers) {
			containers = selectDockerContainers(containers, available)
		}
	}
	containers = sortDockerContainers(containers, s.Containers, mode)

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
	for _, row := range buildSystemRows(s, w, false) {
		addn(row)
	}
	addn(white + bottom(w) + reset)

	addn("")
	addn(white + "┌── Disk Usage" + reset)
	for _, row := range buildDiskRows(s.DiskUsage, false) {
		addn(row)
	}
	addn(white + bottom(w) + reset)

	addn("")
	addn(white + "┌── Health" + reset)
	for _, row := range buildHealthRows(s, false) {
		addn(row)
	}
	addn(white + bottom(w) + reset)

	addn("")
	addn(white + "┌── Docker Services" + reset)
	for _, row := range buildDockerRows(containers, w, false, mode) {
		addn(row)
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
