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
		line := plainTerminalLine(raw)
		if !strings.Contains(line, "NAMES") || !strings.Contains(line, "RAM") || !strings.Contains(line, "STATUS") {
			continue
		}
		ramX := runeIndex(line, "RAM") + 1
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
		line := plainTerminalLine(raw)
		if strings.Contains(line, "NAMES↑") || strings.Contains(line, "alpha") || strings.Contains(line, "beta") {
			if got := len([]rune(line)); got != wantWidth {
				t.Fatalf("landscape row width = %d, want %d: %q", got, wantWidth, line)
			}
		}
	}
}
