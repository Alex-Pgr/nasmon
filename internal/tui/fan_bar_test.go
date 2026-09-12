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

func assertMetricGridAligned(t *testing.T, out string, labels []string) {
	t.Helper()
	wantOpen, wantClose, wantSuffix := -1, -1, -1
	for _, label := range labels {
		line := metricLine(out, label)
		if line == "" {
			t.Fatalf("%s metric line missing", label)
		}
		open := runeIndex(line, "[")
		close := runeIndex(line, "]")
		suffix := close + 2 // closing bracket, exactly one blank, then suffix column
		if wantOpen < 0 {
			wantOpen, wantClose, wantSuffix = open, close, suffix
			continue
		}
		if open != wantOpen || close != wantClose || suffix != wantSuffix {
			t.Fatalf("metric grid not aligned for %s: open=%d/%d close=%d/%d suffix=%d/%d line=%q", label, open, wantOpen, close, wantClose, suffix, wantSuffix, line)
		}
	}
}

func testMetricSnapshot() model.Snapshot {
	rpm := 2200
	gpu := 37
	cpuTemp := 45.0
	gpuTemp := 41.0
	return model.Snapshot{
		FanRPM:          &rpm,
		CPUUsage:        25,
		CPUTempC:        &cpuTemp,
		GPUUsage:        &gpu,
		GPUTempC:        &gpuTemp,
		GPUVCN:          "IDLE",
		MemUsedBytes:    4 << 30,
		MemTotalBytes:   16 << 30,
		MemPercent:      25,
		ZRAMUsedBytes:   1 << 30,
		ZRAMTotalBytes:  4 << 30,
		ZRAMPercent:     25,
	}
}

func TestSystemBarsUseAvailableRegularWidth(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 1}
	r := Renderer{Config: cfg}
	s := testMetricSnapshot()

	out60 := r.RenderInteractive(s, 60, 60, DockerSortDefault)
	labels := []string{"CPU:", "GPU:", "RAM:", "ZRAM:"}
	for _, label := range labels {
		line := metricLine(out60, label)
		if line == "" {
			t.Fatalf("%s line missing", label)
		}
		if got := len([]rune(line)); got != 59 {
			t.Fatalf("%s line width = %d, want 59 (one cell before 60-col edge): %q", label, got, line)
		}
	}
	assertMetricGridAligned(t, out60, labels)

	out80 := r.RenderInteractive(s, 60, 80, DockerSortDefault)
	for _, label := range labels {
		narrow := progressWidth(metricLine(out60, label))
		wide := progressWidth(metricLine(out80, label))
		if wide <= narrow {
			t.Fatalf("%s bar did not grow with terminal: narrow=%d wide=%d", label, narrow, wide)
		}
	}
	assertMetricGridAligned(t, out80, labels)
}

func TestSystemBarsLeaveOneCellBeforeLandscapeBorder(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 1}
	r := Renderer{Config: cfg}
	s := testMetricSnapshot()

	out := r.RenderInteractive(s, 30, 120, DockerSortDefault)
	labels := []string{"CPU", "GPU", "RAM", "ZRAM"}
	assertMetricGridAligned(t, out, labels)
	for _, label := range labels {
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

func TestGPUProgressBarAndSuffix(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 1}
	r := Renderer{Config: cfg}
	out := r.RenderInteractive(testMetricSnapshot(), 60, 70, DockerSortDefault)
	line := metricLine(out, "GPU:")
	if line == "" || progressWidth(line) == 0 {
		t.Fatalf("GPU progress bar missing: %q", line)
	}
	if !strings.Contains(line, "37%") || !strings.Contains(line, "VCN IDLE") {
		t.Fatalf("GPU percent or VCN suffix missing: %q", line)
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
