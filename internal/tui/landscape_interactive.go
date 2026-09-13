package tui

import (
	"fmt"
	"strings"
	"time"

	"nasmon/internal/model"
)

const ultraCompactMaxRows = 16

func landscapeSystemRows(s model.Snapshot) int {
	rows := 9 // uptime, CPU, GPU, RAM, swap, load, net, traffic, disk
	if s.ZRAMTotalBytes > 0 {
		rows++
	}
	return rows
}

func landscapeDiskWidths(usage []model.DiskUsage) (pathW, usedW, totalW int) {
	pathW, usedW, totalW = 1, 1, 1
	for _, d := range usage {
		if n := cellWidth(d.Path); n > pathW {
			pathW = n
		}
		if n := cellWidth(humanBytes(d.UsedBytes)); n > usedW {
			usedW = n
		}
		if n := cellWidth(humanBytes(d.TotalBytes)); n > totalW {
			totalW = n
		}
	}
	pathW = clamp(pathW, 1, 18)
	return
}

func dockerViewportTitle(total, offset, visible int) string {
	if total == 0 || visible >= total {
		return "Docker"
	}
	start := offset + 1
	end := offset + visible
	if end > total {
		end = total
	}
	prefix, suffix := "", ""
	if offset > 0 {
		prefix = "↑ "
	}
	if end < total {
		suffix = " ↓"
	}
	return fmt.Sprintf("Docker %s%d–%d/%d%s", prefix, start, end, total, suffix)
}

func ultraCompactDockerPageSize(total, rows int) int {
	page := rows - 5 // title + summary + Docker box top/header/bottom
	if page < 0 {
		page = 0
	}
	if page > total {
		page = total
	}
	return page
}

func ultraCompactSummary(s model.Snapshot) string {
	ram := fmt.Sprintf("%.1f/%.1fG", float64(s.MemUsedBytes)/(1<<30), float64(s.MemTotalBytes)/(1<<30))
	swap := "-"
	if s.SwapTotalBytes > 0 {
		swap = fmt.Sprintf("%.1f/%.1fG", float64(s.SwapUsedBytes)/(1<<30), float64(s.SwapTotalBytes)/(1<<30))
	}
	return fmt.Sprintf("CPU %d%%   RAM %s   SWAP %s", s.CPUUsage, ram, swap)
}

func (r Renderer) renderUltraCompact(s model.Snapshot, w, rows int, mode DockerSortMode, dockerOffset int) string {
	page := ultraCompactDockerPageSize(len(s.Containers), rows)
	sorted := sortDockerContainers(s.Containers, s.Containers, mode)
	offset := ClampDockerOffset(dockerOffset, len(sorted), page)
	visible := sorted
	if page < len(sorted) {
		if page > 0 {
			visible = sorted[offset : offset+page]
		} else {
			visible = nil
		}
	}

	var b strings.Builder
	title := fmt.Sprintf("NAS Health Monitor %s", r.Config.MainInterval) + staleSuffix(s)
	add(&b, cyan+center(title, w)+reset)
	add(&b, white+center(ultraCompactSummary(s), w)+reset)

	docker := buildDockerRows(visible, w-2, true, mode)
	dockerTitle := dockerViewportTitle(len(s.Containers), offset, len(visible)) + collectorTitleSuffix(s.DockerCollector, r.Config.DockerInterval)
	for _, row := range makeBox(w, dockerTitle, docker, len(docker)) {
		add(&b, white+row+reset)
	}
	return b.String()
}

func (r Renderer) renderLandscape(s model.Snapshot, w, rows int, mode DockerSortMode, dockerOffset int) string {
	healthRows := len(buildHealthRows(s, true)) + len(r.collectorWarningRows(s, true))
	lowerRows := len(s.DiskUsage)
	if healthRows > lowerRows {
		lowerRows = healthRows
	}

	upperRows := landscapeSystemRows(s)
	if dockerRows := 1 + len(s.Containers); dockerRows > upperRows {
		upperRows = dockerRows
	}
	fullRows := 8 + upperRows + lowerRows
	compactHeader := rows > 0 && fullRows > rows

	copySnap := s
	sorted := sortDockerContainers(s.Containers, s.Containers, mode)
	visible := len(sorted)
	offset := 0
	if compactHeader && rows > 0 {
		visible = rows - 7 - lowerRows
		if visible < 0 {
			visible = 0
		}
		if visible > len(sorted) {
			visible = len(sorted)
		}
		offset = ClampDockerOffset(dockerOffset, len(sorted), visible)
		if visible > 0 {
			sorted = sorted[offset : offset+visible]
		} else {
			sorted = nil
		}
	}
	copySnap.Containers = sorted

	return r.layoutLandscape(copySnap, w, mode, compactHeader, len(s.Containers), offset, visible)
}

func (r Renderer) layoutLandscape(s model.Snapshot, w int, mode DockerSortMode, compactHeader bool, dockerTotal, dockerOffset, dockerVisible int) string {
	var b strings.Builder
	if compactHeader {
		title := fmt.Sprintf("NAS Health Monitor %s", r.Config.MainInterval) + staleSuffix(s)
		add(&b, cyan+center(title, w)+reset)
	} else {
		add(&b, cyan+full(w)+reset)
		title := fmt.Sprintf("NAS Health Monitor • %s • %s", time.Now().Format("15:04:05"), r.Config.MainInterval) + staleSuffix(s)
		add(&b, white+center(title, w)+reset)
		add(&b, cyan+full(w)+reset)
	}

	const gap = 2
	lw := (w - gap) / 2
	rw := w - gap - lw

	system := buildSystemRows(s, lw-3, true)
	docker := buildDockerRows(s.Containers, rw-2, true, mode)

	upperRows := len(system)
	if len(docker) > upperRows {
		upperRows = len(docker)
	}
	leftUpper := makeBox(lw, "System", system, upperRows)
	dockerTitle := dockerViewportTitle(dockerTotal, dockerOffset, dockerVisible) + collectorTitleSuffix(s.DockerCollector, r.Config.DockerInterval)
	rightUpper := makeBox(rw, dockerTitle, docker, upperRows)
	for _, row := range composeBoxRows(leftUpper, rightUpper, lw, rw, gap) {
		add(&b, white+row+reset)
	}

	add(&b, "")

	disks := buildDiskRows(s.DiskUsage, true)
	health := r.collectorWarningRows(s, true)
	health = append(health, buildHealthRows(s, true)...)
	lowerRows := len(disks)
	if len(health) > lowerRows {
		lowerRows = len(health)
	}
	leftLower := makeBox(lw, "Disk Usage", disks, lowerRows)
	rightLower := makeBox(rw, "Health", health, lowerRows)
	for _, row := range composeBoxRows(leftLower, rightLower, lw, rw, gap) {
		add(&b, white+row+reset)
	}

	return b.String()
}
