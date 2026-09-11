package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"nasmon/internal/model"
)

// renderLandscapeInteractiveAdaptive mirrors the adaptive landscape sizing,
// but keeps Docker filtering and user sorting as separate steps: first choose
// the important rows that fit, then sort only that visible subset.
func (r Renderer) renderLandscapeInteractiveAdaptive(s model.Snapshot, w, rows int, mode DockerSortMode) string {
	lowerRows := len(s.DiskUsage)
	if h := 1 + len(s.DiskHealth); h > lowerRows {
		lowerRows = h
	}

	upperRows := 7
	if dockerRows := 1 + len(s.Containers); dockerRows > upperRows { // +1 for column header
		upperRows = dockerRows
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

	out := r.renderLandscapeInteractive(copySnap, w, mode)
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

func (r Renderer) renderLandscapeInteractive(s model.Snapshot, w int, mode DockerSortMode) string {
	var b strings.Builder
	add(&b, cyan+full(w)+reset)
	add(&b, white+center(fmt.Sprintf("NAS Health Monitor • %s • %s", time.Now().Format("15:04:05"), r.Config.MainInterval), w)+reset)
	add(&b, cyan+full(w)+reset)

	const gap = 2
	lw := (w - gap) / 2
	rw := w - gap - lw

	traffic := "Traffic measuring..."
	if s.NetReady {
		traffic = fmt.Sprintf("Traffic RX %s TX %s", speed(s.RXBps), speed(s.TXBps))
	}
	dio := "Disk IO measuring..."
	if s.DiskIOReady {
		dio = fmt.Sprintf("Disk R %s W %s", speed(s.DiskReadBps), speed(s.DiskWriteBps))
	}
	netLabel := "No IP"
	if s.IP != "" {
		netLabel = s.IP
	}
	vcnColor := gray
	if s.GPUVCN == "ACTIVE" {
		vcnColor = green
	}
	ramTail := fmt.Sprintf("%s/%sG", gib(s.MemUsedBytes), gib(s.MemTotalBytes))
	landscapeBW := clamp(lw-(20+utf8.RuneCountInString(ramTail)), 6, 26)

	system := []string{
		fmt.Sprintf(" %sCPU%s  %s%-5s%s  %s%-4s%s%s[%s]%s", white, reset, tempColor(s.CPUTempC), temp(s.CPUTempC), reset, pctColor(s.CPUUsage, "cpu"), fmt.Sprintf("%d%%", s.CPUUsage), reset, gray, bar(s.CPUUsage, landscapeBW), reset),
		fmt.Sprintf(" %sGPU%s  %s%-5s%s  %s %sVCN%s %s%s%s", white, reset, lightGray, temp(s.GPUTempC), reset, gpuUsageLabel(s.GPUUsage), white, reset, vcnColor, s.GPUVCN, reset),
		fmt.Sprintf(" %sRAM%s         %s%-4s%s%s[%s]%s %s%s%s", white, reset, pctColor(s.MemPercent, "mem"), fmt.Sprintf("%d%%", s.MemPercent), reset, gray, bar(s.MemPercent, landscapeBW), reset, gray, ramTail, reset),
		fmt.Sprintf(" %sLoad%s %s%s%s %s(%s, %s)%s", white, reset, lightGray, s.Load1, reset, gray, s.Load5, s.Load15, reset),
		fmt.Sprintf(" %sNet%s  %s%s%s", white, reset, blue, netLabel, reset),
		fmt.Sprintf(" %sTraffic%s%s%s", white, reset, gray, strings.TrimPrefix(traffic, "Traffic")+reset),
		fmt.Sprintf(" %sDisk%s%s%s", white, reset, gray, strings.TrimPrefix(dio, "Disk")+reset),
	}

	const ramW = 6
	inner := rw - 2
	nameW := clamp(rw/3, 10, 24)
	statusW := inner - nameW - ramW - 7
	if statusW < 6 {
		nameW -= 6 - statusW
		if nameW < 8 {
			nameW = 8
		}
		statusW = inner - nameW - ramW - 7
	}
	if statusW < 1 {
		statusW = 1
	}
	namesLabel, ramLabel := dockerSortLabels(mode)
	docker := make([]string, 0, len(s.Containers)+1)
	docker = append(docker, fmt.Sprintf("   %-*s  %*s  STATUS", nameW, namesLabel, ramW, ramLabel))
	for _, c := range s.Containers {
		icon, color := "●", blue
		if c.State != "running" || c.Health == "unhealthy" {
			icon, color = "⚠", yellow
		} else if c.Health == "healthy" {
			icon, color = "✓", green
		}
		status := dockerStatus(c)
		if status == "" {
			status = c.State
		}
		status += fmt.Sprintf(" R:%d", c.Restarts)
		docker = append(docker, fmt.Sprintf(" %s%s%s %s%-*s%s  %s%*s%s  %s%s%s", color, icon, reset, lightGray, nameW, trunc(c.Name, nameW), reset, gray, ramW, dockerMemoryLabel(c.MemoryBytes), reset, color, trunc(status, statusW), reset))
	}

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

	disks := make([]string, 0, len(s.DiskUsage))
	for _, d := range s.DiskUsage {
		disks = append(disks, fmt.Sprintf(" %s%-12s%s %s%s/%s%s %s%d%%%s", lightGray, d.Path, reset, white, humanBytes(d.UsedBytes), humanBytes(d.TotalBytes), reset, pctColor(d.Percent, "disk"), d.Percent, reset))
	}

	health := []string{}
	if s.FailedUnits == 0 {
		health = append(health, fmt.Sprintf(" %ssystemd OK (0 failed)%s", green, reset))
	} else {
		health = append(health, fmt.Sprintf(" %ssystemd WARNING (%d failed)%s", yellow, s.FailedUnits, reset))
	}
	for _, h := range s.DiskHealth {
		state := "SMART " + h.Health
		stateColor := yellow
		if diskHealthOK(h) {
			state = "SMART OK"
			stateColor = green
		}
		sleepMark := ""
		if h.Sleeping {
			sleepMark = " " + blue + "SLEEP" + reset
		}
		health = append(health, fmt.Sprintf(" %s%s%s %s%s %s%s%s %s%s%s%s", lightGray, h.Device, reset, white, h.Temperature, stateColor, state, reset, gray, diskHealthDetails(h, true), reset, sleepMark))
	}

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
