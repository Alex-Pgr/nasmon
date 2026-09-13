package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"nasmon/internal/model"
)

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
		if n := utf8.RuneCountInString(d.Path); n > pathW {
			pathW = n
		}
		if n := utf8.RuneCountInString(humanBytes(d.UsedBytes)); n > usedW {
			usedW = n
		}
		if n := utf8.RuneCountInString(humanBytes(d.TotalBytes)); n > totalW {
			totalW = n
		}
	}
	pathW = clamp(pathW, 1, 18)
	return
}

// renderLandscape selects the visible Docker subset before sorting, then lays
// out the already-built section rows without post-render ANSI rewriting.
func (r Renderer) renderLandscape(s model.Snapshot, w, rows int, mode DockerSortMode) string {
	lowerRows := len(s.DiskUsage)
	if h := 1 + len(s.DiskHealth); h > lowerRows {
		lowerRows = h
	}

	upperRows := landscapeSystemRows(s)
	if dockerRows := 1 + len(s.Containers); dockerRows > upperRows { // +1 for column header
		upperRows = dockerRows
	}
	fullRows := 8 + upperRows + lowerRows
	compactHeader := rows > 0 && fullRows > rows

	copySnap := s
	copySnap.Containers = append([]model.Container(nil), s.Containers...)
	if compactHeader && rows > 0 {
		// Compact landscape has one global header row, two rows of box borders,
		// one spacer and two lower-box borders. Docker also needs one table header.
		availableContainers := rows - 7 - lowerRows
		if availableContainers < 0 {
			availableContainers = 0
		}
		if availableContainers < len(copySnap.Containers) {
			copySnap.Containers = selectDockerContainers(copySnap.Containers, availableContainers)
		}
	}
	copySnap.Containers = sortDockerContainers(copySnap.Containers, s.Containers, mode)

	out := r.layoutLandscape(copySnap, w, mode)
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

func (r Renderer) layoutLandscape(s model.Snapshot, w int, mode DockerSortMode) string {
	var b strings.Builder
	add(&b, cyan+full(w)+reset)
	add(&b, white+center(fmt.Sprintf("NAS Health Monitor • %s • %s", time.Now().Format("15:04:05"), r.Config.MainInterval), w)+reset)
	add(&b, cyan+full(w)+reset)

	const gap = 2
	lw := (w - gap) / 2
	rw := w - gap - lw

	// makeBox has an inner width of lw-2. Metric rows are one cell shorter so
	// the fixed suffix column is followed by exactly one blank before the System
	// border. CPU/GPU/RAM/ZRAM all share the same bar start/end and suffix start.
	system := buildSystemRows(s, lw-3, true)
	docker := buildDockerRows(s.Containers, rw-2, true, mode)

	upperRows := len(system)
	if len(docker) > upperRows {
		upperRows = len(docker)
	}
	leftUpper := makeBox(lw, "System", system, upperRows)
	rightUpper := makeBox(rw, "Docker", docker, upperRows)
	for _, row := range composeBoxRows(leftUpper, rightUpper, lw, rw, gap) {
		add(&b, white+row+reset)
	}

	add(&b, "")

	disks := buildDiskRows(s.DiskUsage, true)
	health := buildHealthRows(s, true)
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
