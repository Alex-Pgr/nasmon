package tui

import (
	"strings"
	"testing"
	"time"

	"nasmon/internal/app"
	"nasmon/internal/model"
)

func metricLine(out, label string) string {
	for _, raw := range strings.Split(out, "\n") {
		plain := plainTerminalLine(raw)
		if strings.Contains(plain, label) && strings.Contains(plain, "[") && strings.Contains(plain, "]") {
			return plain
		}
	}
	return ""
}

func progressWidth(line string) int {
	open := runeIndex(line, "[")
	close := runeIndex(line, "]")
	if open < 0 || close <= open {
		return 0
	}
	return close - open - 1
}

func TestSystemBarsUseAvailableRegularWidth(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 1}
	r := Renderer{Config: cfg}
	rpm := 2200
	s := model.Snapshot{
		FanRPM:          &rpm,
		CPUUsage:        25,
		MemUsedBytes:    4 << 30,
		MemTotalBytes:   16 << 30,
		MemPercent:      25,
		ZRAMUsedBytes:   1 << 30,
		ZRAMTotalBytes:  4 << 30,
		ZRAMPercent:     25,
	}

	out60 := r.RenderInteractive(s, 60, 60, DockerSortDefault)
	for _, label := range []string{"CPU:", "RAM:", "ZRAM:"} {
		line := metricLine(out60, label)
		if line == "" {
			t.Fatalf("%s line missing", label)
		}
		if got := len([]rune(line)); got != 59 {
			t.Fatalf("%s line width = %d, want 59 (one cell before 60-col edge): %q", label, got, line)
		}
	}

	out80 := r.RenderInteractive(s, 60, 80, DockerSortDefault)
	for _, label := range []string{"CPU:", "RAM:", "ZRAM:"} {
		narrow := progressWidth(metricLine(out60, label))
		wide := progressWidth(metricLine(out80, label))
		if wide <= narrow {
			t.Fatalf("%s bar did not grow with terminal: narrow=%d wide=%d", label, narrow, wide)
		}
	}
}

func TestSystemBarsLeaveOneCellBeforeLandscapeBorder(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 1}
	r := Renderer{Config: cfg}
	rpm := 2200
	s := model.Snapshot{
		FanRPM:          &rpm,
		CPUUsage:        50,
		MemUsedBytes:    4 << 30,
		MemTotalBytes:   16 << 30,
		MemPercent:      25,
		ZRAMUsedBytes:   1 << 30,
		ZRAMTotalBytes:  4 << 30,
		ZRAMPercent:     25,
	}

	out := r.RenderInteractive(s, 30, 120, DockerSortDefault)
	for _, label := range []string{"CPU", "RAM", "ZRAM"} {
		line := metricLine(out, label)
		if line == "" {
			t.Fatalf("%s landscape line missing", label)
		}
		border := nthRuneIndex(line, '│', 2)
		if border < 2 {
			t.Fatalf("%s second border missing: %q", label, line)
		}
		runes := []rune(line)
		if runes[border-1] != ' ' {
			t.Fatalf("%s must leave one blank before System border: %q", label, line)
		}
		if runes[border-2] == ' ' {
			t.Fatalf("%s leaves more than one blank before System border: %q", label, line)
		}
	}
}

func TestFanRPMUsesGrayStyle(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 1}
	r := Renderer{Config: cfg}
	rpm := 2200
	out := r.RenderInteractive(model.Snapshot{FanRPM: &rpm}, 60, 60, DockerSortDefault)
	if !strings.Contains(out, gray+"Fan 2200 rpm"+reset) {
		t.Fatalf("fan RPM is not rendered with gray style")
	}
}
