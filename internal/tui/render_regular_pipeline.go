package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Alex-Pgr/nasmon/internal/model"
)

// renderRegular is the canonical regular layout. It sorts Docker rows before
// applying the viewport so keyboard scrolling works in portrait layouts too.
func (r Renderer) renderRegular(s model.Snapshot, w, rows int, mode DockerSortMode, dockerOffset int) string {
	collectorWarnings := r.collectorWarningRows(s, false)
	fullRows := regularFixedRows(s, 5) + len(collectorWarnings) + len(s.Containers)
	compactHeader := rows > 0 && fullRows > rows

	page := r.regularDockerPageSize(s, rows)
	sorted := sortDockerContainers(s.Containers, s.Containers, mode)
	offset := ClampDockerOffset(dockerOffset, len(sorted), page)
	containers := sorted
	if page < len(sorted) {
		if page > 0 {
			containers = sorted[offset : offset+page]
		} else {
			containers = nil
		}
	}

	var b strings.Builder
	n := 0
	addn := func(x string) {
		add(&b, truncateCells(x, w))
		n++
	}
	if compactHeader {
		title := fmt.Sprintf("NAS Health Monitor %s", r.Config.MainInterval) + staleSuffix(s)
		addn(cyan + center(title, w) + reset)
	} else {
		info := fmt.Sprintf("%s • Обновление: %s", s.UpdatedAt.Format("2006-01-02 15:04:05"), r.Config.MainInterval)
		addn(cyan + full(w) + reset)
		addn(white + center("NAS Health Monitor"+staleSuffix(s), w) + reset)
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
	for _, row := range collectorWarnings {
		addn(row)
	}
	for _, row := range buildHealthRows(s, false) {
		addn(row)
	}
	addn(white + bottom(w) + reset)

	addn("")
	viewportTitle := dockerViewportTitle(len(s.Containers), offset, len(containers))
	if viewportTitle == "Docker" {
		viewportTitle = "Docker Services"
	} else {
		viewportTitle = strings.Replace(viewportTitle, "Docker", "Docker Services", 1)
	}
	dockerTitle := "┌── " + viewportTitle + collectorTitleSuffix(s.DockerCollector, r.Config.DockerInterval)
	addn(white + dockerTitle + reset)
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
