package tui

import (
	"fmt"
	"strings"
	"time"

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

// renderLandscape selects the visible Docker subset before sorting and passes
// the chosen header mode into the canonical landscape composer. Compact mode
// is therefore composed directly rather than rewriting an already-rendered
// ANSI frame.
func (r Renderer) renderLandscape(s model.Snapshot, w, rows int, mode DockerSortMode) string {
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
	copySnap.Containers = append([]model.Container(nil), s.Containers...)
	if compactHeader && rows > 0 {
		availableContainers := rows - 7 - lowerRows
		if availableContainers < 0 {
			availableContainers = 0
		}
		if availableContainers < len(copySnap.Containers) {
			copySnap.Containers = selectDockerContainers(copySnap.Containers, availableContainers)
		}
	}
	copySnap.Containers = sortDockerContainers(copySnap.Containers, s.Containers, mode)

	return r.layoutLandscape(copySnap, w, mode, compactHeader)
}

func (r Renderer) layoutLandscape(s model.Snapshot, w int, mode DockerSortMode, compactHeader bool) string {
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
	dockerTitle := "Docker" + collectorTitleSuffix(s.DockerCollector, r.Config.DockerInterval)
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
