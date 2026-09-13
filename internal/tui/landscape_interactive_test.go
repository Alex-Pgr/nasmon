package tui

import (
	"strings"
	"testing"
	"time"

	"nasmon/internal/app"
	"nasmon/internal/model"
)

func TestLandscapeInteractiveDockerColumnsAndSort(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{Containers: []model.Container{
		{Name: "small", State: "running", Status: "Up 1 hour (healthy)", Health: "healthy", MemoryBytes: 100 << 20},
		{Name: "big", State: "running", Status: "Up 1 hour (healthy)", Health: "healthy", MemoryBytes: 900 << 20},
	}}

	out := r.RenderInteractive(s, 30, 120, DockerSortRAMDesc)
	if !strings.Contains(out, "NAMES") || !strings.Contains(out, "RAM↓") || !strings.Contains(out, "STATUS") {
		t.Fatalf("landscape Docker columns or RAM sort marker missing")
	}
	if strings.Contains(out, "(healthy)") {
		t.Fatalf("health suffix should not be rendered in Docker status")
	}
	if strings.Index(out, "big") > strings.Index(out, "small") {
		t.Fatalf("RAM descending order not applied")
	}
}

func TestLandscapeInteractiveDockerHeaderIsClickable(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{Containers: []model.Container{{Name: "one", State: "running", Status: "Up 1h", MemoryBytes: 10}}}
	out := r.RenderInteractive(s, 30, 120, DockerSortDefault)

	lines := strings.Split(out, "\n")
	for i, raw := range lines {
		line := stripANSI(raw)
		if !strings.Contains(line, "NAMES") || !strings.Contains(line, "RAM") || !strings.Contains(line, "STATUS") {
			continue
		}
		ramX := cellIndex(line, "RAM") + 1
		mode, changed := DockerSortForClick(out, ramX, i+1, DockerSortDefault)
		if !changed || mode != DockerSortRAMDesc {
			t.Fatalf("landscape RAM click = %v, changed=%v", mode, changed)
		}
		return
	}
	t.Fatalf("landscape Docker header not found")
}

func TestLandscapeInteractiveRowsKeepSameWidth(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{Containers: []model.Container{
		{Name: "alpha", State: "running", Status: "Up 1h", MemoryBytes: 100 << 20},
		{Name: "beta", State: "running", Status: "Up 1h", MemoryBytes: 200 << 20},
	}}
	out := r.RenderInteractive(s, 30, 120, DockerSortNameAsc)

	wantWidth := 118
	for _, raw := range strings.Split(out, "\n") {
		line := stripANSI(raw)
		if strings.Contains(line, "NAMES↑") || strings.Contains(line, "alpha") || strings.Contains(line, "beta") {
			if got := cellWidth(line); got != wantWidth {
				t.Fatalf("landscape row width = %d, want %d: %q", got, wantWidth, line)
			}
		}
	}
}

func TestLandscapeInteractiveShowsVerticalSystemMetrics(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{
		Uptime:         4*time.Hour + 12*time.Minute,
		MemUsedBytes:   5 << 30,
		MemTotalBytes:  16 << 30,
		MemPercent:     31,
		ZRAMUsedBytes:  1 << 30,
		ZRAMTotalBytes: 4 << 30,
		ZRAMPercent:    25,
		SwapUsedBytes:  1 << 30,
		SwapTotalBytes: 12 << 30,
		SwapPercent:    8,
		IOWait:         3,
	}
	out := r.RenderInteractive(s, 30, 120, DockerSortDefault)
	for _, want := range []string{"Uptime", "4h 12m", "ZRAM", "1.0/4.0G", "Swap", "1.0/12.0G", "IOwait", "3%"} {
		if !strings.Contains(out, want) {
			t.Fatalf("landscape system metric %q missing", want)
		}
	}
}

func TestLandscapeDiskUsageColumnsAlign(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{DiskUsage: []model.DiskUsage{
		{Path: "/", UsedBytes: 9 << 30, TotalBytes: 40 << 30, Percent: 23},
		{Path: "/mnt/fast", UsedBytes: 123 << 30, TotalBytes: 468 << 30, Percent: 26},
	}}
	out := r.RenderInteractive(s, 30, 120, DockerSortDefault)

	var slashCols, pctCols []int
	for _, raw := range strings.Split(out, "\n") {
		line := stripANSI(raw)
		switch {
		case strings.Contains(line, "9.0G/40.0G"):
			slashCols = append(slashCols, cellIndex(line, "/40.0G"))
			pctCols = append(pctCols, cellIndex(line, "23%"))
		case strings.Contains(line, "123.0G/468.0G"):
			slashCols = append(slashCols, cellIndex(line, "/468.0G"))
			pctCols = append(pctCols, cellIndex(line, "26%"))
		}
	}
	if len(slashCols) != 2 || len(pctCols) != 2 {
		t.Fatalf("disk usage rows not found: slash=%v pct=%v", slashCols, pctCols)
	}
	if slashCols[0] != slashCols[1] {
		t.Fatalf("used/total separators not aligned: %v", slashCols)
	}
	if pctCols[0] != pctCols[1] {
		t.Fatalf("percent columns not aligned: %v", pctCols)
	}
}
