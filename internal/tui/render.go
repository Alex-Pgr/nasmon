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

func add(b *strings.Builder, s string) {
	b.WriteString("\033[2K\r")
	b.WriteString(s)
	b.WriteByte('\n')
}

func bottom(w int) string { return "└" + rep("─", w-1) }
func full(w int) string   { return rep("═", w) }

func diskHealthOK(h model.DiskHealth) bool {
	if h.Health != "OK" {
		return false
	}
	if h.NVMe {
		if h.CriticalWarning != 0 || h.MediaErrors > 0 || (h.NVMeMetrics && h.PercentageUsed >= 100) {
			return false
		}
		return h.SpareThreshold == 0 || h.AvailableSpare >= h.SpareThreshold
	}
	return h.Reallocated == 0 && h.Pending == 0 && h.Uncorrect == 0
}

func diskHealthDetails(h model.DiskHealth, compact bool) string {
	if !h.NVMe {
		return fmt.Sprintf("R:%d P:%d U:%d", h.Reallocated, h.Pending, h.Uncorrect)
	}
	if !h.NVMeMetrics {
		return "NVMe metrics N/A"
	}

	var details string
	if compact {
		details = fmt.Sprintf("W:%d%% S:%d%%", h.PercentageUsed, h.AvailableSpare)
	} else {
		details = fmt.Sprintf("Wear:%d%% Spare:%d%%", h.PercentageUsed, h.AvailableSpare)
	}
	if h.MediaErrors != 0 {
		details += fmt.Sprintf(" Media:%d", h.MediaErrors)
	}
	if h.ErrorLogEntries != 0 {
		details += fmt.Sprintf(" Err:%d", h.ErrorLogEntries)
	}
	if h.CriticalWarning != 0 {
		details += fmt.Sprintf(" CW:0x%02x", h.CriticalWarning)
	}
	return details
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
