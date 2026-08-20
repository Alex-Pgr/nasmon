package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"nasmon/internal/app"
	"nasmon/internal/model"
)

const (
	white     = "\033[0;97m"
	lightGray = "\033[0;37m"
	gray      = "\033[0;90m"
	cyan      = "\033[0;36m"
	blue      = "\033[0;34m"
	green     = "\033[0;32m"
	yellow    = "\033[0;33m"
	orange    = "\033[0;38;5;214m"
	reset     = "\033[0m"
)

type Renderer struct{ Config app.Config }

func clamp(x, a, b int) int {
	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}
func rep(s string, n int) string {
	if n < 0 {
		n = 0
	}
	return strings.Repeat(s, n)
}
func trunc(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}
func center(s string, w int) string {
	l := utf8.RuneCountInString(s)
	if l >= w {
		return trunc(s, w)
	}
	return rep(" ", (w-l)/2) + s
}
func pctColor(p int, kind string) string {
	switch kind {
	case "cpu":
		if p > 90 {
			return orange
		}
		if p > 70 {
			return yellow
		}
		if p > 30 {
			return blue
		}
	case "mem":
		if p > 90 {
			return orange
		}
		if p > 75 {
			return yellow
		}
		if p > 50 {
			return blue
		}
	case "disk":
		if p >= 90 {
			return orange
		}
		if p >= 80 {
			return yellow
		}
		if p >= 60 {
			return blue
		}
	case "io":
		if p >= 40 {
			return orange
		}
		if p >= 15 {
			return yellow
		}
		if p >= 5 {
			return blue
		}
	}
	return lightGray
}
func tempColor(t *float64) string {
	if t == nil {
		return gray
	}
	if *t > 80 {
		return orange
	}
	if *t > 70 {
		return yellow
	}
	if *t > 60 {
		return blue
	}
	return lightGray
}
func temp(t *float64) string {
	if t == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.0f°C", *t)
}
func bar(p, w int) string {
	p = clamp(p, 0, 100)
	w = clamp(w, 1, 80)
	f := p * w / 100
	return rep("█", f) + rep("░", w-f)
}
func gib(b uint64) string { return fmt.Sprintf("%.1f", float64(b)/(1<<30)) }
func humanBytes(b uint64) string {
	switch {
	case b >= 1<<40:
		return fmt.Sprintf("%.1fT", float64(b)/(1<<40))
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
func speed(b int64) string {
	if b < 0 {
		b = 0
	}
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GB/s", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB/s", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB/s", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B/s", b)
	}
}
func dur(d time.Duration) string {
	t := int64(d.Seconds())
	if t < 60 {
		return fmt.Sprintf("%ds", t)
	}
	days := t / 86400
	t %= 86400
	h := t / 3600
	t %= 3600
	m := t / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
func line(s string) *strings.Builder { return &strings.Builder{} }

func (r Renderer) Render(s model.Snapshot, rows, cols int) string {
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
	landscape := rows <= 30 && cols >= 80
	if landscape {
		return r.renderLandscape(s, draw)
	}
	return r.renderRegular(s, draw, rows)
}
func add(b *strings.Builder, s string) {
	b.WriteString("\033[2K\r")
	b.WriteString(s)
	b.WriteByte('\n')
}
func bottom(w int) string { return "└" + rep("─", w-1) }
func full(w int) string   { return rep("═", w) }

func (r Renderer) renderRegular(s model.Snapshot, w, rows int) string {
	var b strings.Builder
	n := 0
	addn := func(x string) { add(&b, x); n++ }
	info := fmt.Sprintf("%s • Обновление: %s", s.UpdatedAt.Format("2006-01-02 15:04:05"), r.Config.MainInterval)
	addn(cyan + full(w) + reset)
	addn(white + center("NAS Health Monitor", w) + reset)
	addn(gray + center(trunc(info, w), w) + reset)
	addn(cyan + full(w) + reset)
	addn("")
	addn(white + "┌── System Status" + reset)
	addn(fmt.Sprintf("%s│ Uptime:%s %s%s%s", white, reset, lightGray, dur(s.Uptime), reset))
	ramTail := fmt.Sprintf("%s/%s GiB", gib(s.MemUsedBytes), gib(s.MemTotalBytes))
	bw := clamp(w-(6+1+5+2+4+1+2+utf8.RuneCountInString(ramTail)), 10, 26)
	addn(fmt.Sprintf("%s│ CPU:%s %s%-5s%s  %s%-4s%s%s[%s]%s", white, reset, tempColor(s.CPUTempC), temp(s.CPUTempC), reset, pctColor(s.CPUUsage, "cpu"), fmt.Sprintf("%d%%", s.CPUUsage), reset, gray, bar(s.CPUUsage, bw), reset))
	vcnColor := gray
	if s.GPUVCN == "ACTIVE" {
		vcnColor = green
	}
	addn(fmt.Sprintf("%s│ GPU:%s %s%-5s%s  %sVCN:%s %s%-6s%s", white, reset, lightGray, temp(s.GPUTempC), reset, white, reset, vcnColor, s.GPUVCN, reset))
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
	for _, h := range s.DiskHealth {
		mark := yellow + "SMART " + h.Health + reset
		if h.Sleeping {
			mark = blue + "SLEEP" + reset
		} else if h.Health == "OK" && h.Reallocated == 0 && h.Pending == 0 && h.Uncorrect == 0 {
			mark = green + "✓ SMART OK" + reset
		}
		addn(fmt.Sprintf("%s│ %s/dev/%s%s  %s  %s %sR:%d P:%d U:%d%s", white, lightGray, h.Device, reset, h.Temperature, mark, gray, h.Reallocated, h.Pending, h.Uncorrect, reset))
	}
	addn(white + bottom(w) + reset)
	addn("")
	addn(white + "┌── Docker Services" + reset)
	nameW := clamp(w/3, 10, 30)
	addn(fmt.Sprintf("%s│ %-*s  STATUS%s", white, nameW, "NAMES", reset))
	for _, c := range s.Containers {
		icon, color := "●", blue
		if c.Health == "healthy" {
			icon, color = "✓", green
		} else if c.State != "running" {
			icon, color = "⚠", yellow
		}
		txt := c.Status
		if c.Health != "" && !strings.Contains(txt, "("+c.Health+")") {
			txt += " (" + c.Health + ")"
		}
		txt += fmt.Sprintf(" R:%d", c.Restarts)
		addn(fmt.Sprintf("%s│ %s%s%s %s%-*s%s  %s%s%s", white, color, icon, reset, lightGray, nameW, trunc(c.Name, nameW), reset, color, trunc(txt, w-nameW-7), reset))
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
func boxTop(w int, title string) string {
	if w < 4 {
		return rep("─", w)
	}
	prefix := "┌── " + title + " "
	remain := w - utf8.RuneCountInString(prefix) - 1
	if remain < 1 {
		return trunc(prefix, w-1) + "┐"
	}
	return prefix + rep("─", remain) + "┐"
}

func visibleRunes(s string) int {
	n := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) {
				c := s[i]
				i++
				if c >= '@' && c <= '~' {
					break
				}
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		n++
		i += size
	}
	return n
}

func truncANSI(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if visibleRunes(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}

	var b strings.Builder
	visible := 0
	limit := n - 1
	for i := 0; i < len(s) && visible < limit; {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			start := i
			i += 2
			for i < len(s) {
				c := s[i]
				i++
				if c >= '@' && c <= '~' {
					break
				}
			}
			b.WriteString(s[start:i])
			continue
		}
		rn, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		b.WriteRune(rn)
		visible++
		i += size
	}
	b.WriteRune('…')
	b.WriteString(reset)
	return b.String()
}

func boxLine(w int, content string) string {
	if w < 2 {
		return truncANSI(content, w)
	}
	inner := w - 2
	content = truncANSI(content, inner)
	pad := inner - visibleRunes(content)
	if pad < 0 {
		pad = 0
	}
	return "│" + content + reset + white + rep(" ", pad) + "│"
}

func boxBottom(w int) string {
	if w < 2 {
		return rep("─", w)
	}
	return "└" + rep("─", w-2) + "┘"
}

func composeBoxRows(left, right []string, lw, rw, gap int) []string {
	rows := len(left)
	if len(right) > rows {
		rows = len(right)
	}
	out := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		l := rep(" ", lw)
		r := rep(" ", rw)
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		out = append(out, l+rep(" ", gap)+r)
	}
	return out
}

func makeBox(w int, title string, content []string, dataRows int) []string {
	if dataRows < len(content) {
		dataRows = len(content)
	}
	out := make([]string, 0, dataRows+2)
	out = append(out, boxTop(w, title))
	for i := 0; i < dataRows; i++ {
		line := ""
		if i < len(content) {
			line = content[i]
		}
		out = append(out, boxLine(w, line))
	}
	out = append(out, boxBottom(w))
	return out
}

func (r Renderer) renderLandscape(s model.Snapshot, w int) string {
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
		fmt.Sprintf(" %sGPU%s  %s%-5s%s  %sVCN%s %s%s%s", white, reset, lightGray, temp(s.GPUTempC), reset, white, reset, vcnColor, s.GPUVCN, reset),
		fmt.Sprintf(" %sRAM%s         %s%-4s%s%s[%s]%s %s%s%s", white, reset, pctColor(s.MemPercent, "mem"), fmt.Sprintf("%d%%", s.MemPercent), reset, gray, bar(s.MemPercent, landscapeBW), reset, gray, ramTail, reset),
		fmt.Sprintf(" %sLoad%s %s%s%s %s(%s, %s)%s", white, reset, lightGray, s.Load1, reset, gray, s.Load5, s.Load15, reset),
		fmt.Sprintf(" %sNet%s  %s%s%s", white, reset, blue, netLabel, reset),
		fmt.Sprintf(" %sTraffic%s%s%s", white, reset, gray, strings.TrimPrefix(traffic, "Traffic")+reset),
		fmt.Sprintf(" %sDisk%s%s%s", white, reset, gray, strings.TrimPrefix(dio, "Disk")+reset),
	}

	docker := make([]string, 0, len(s.Containers))
	for _, c := range s.Containers {
		color := blue
		if c.Health == "healthy" {
			color = green
		} else if c.State != "running" {
			color = yellow
		}
		status := c.Status
		if c.Health != "" && !strings.Contains(status, "("+c.Health+")") {
			status += " (" + c.Health + ")"
		}
		if c.Restarts > 0 {
			status += fmt.Sprintf(" R:%d", c.Restarts)
		}
		docker = append(docker, fmt.Sprintf(" %s%s%s: %s%s%s", lightGray, c.Name, reset, color, status, reset))
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
		if h.Sleeping {
			state = "SLEEP"
			stateColor = blue
		} else if h.Health == "OK" && h.Reallocated == 0 && h.Pending == 0 && h.Uncorrect == 0 {
			state = "SMART OK"
			stateColor = green
		}
		health = append(health, fmt.Sprintf(" %s/dev/%s%s %s%s %s%s%s %sR:%d P:%d U:%d%s", lightGray, h.Device, reset, white, h.Temperature, stateColor, state, reset, gray, h.Reallocated, h.Pending, h.Uncorrect, reset))
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
