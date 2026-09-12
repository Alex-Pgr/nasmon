package tui

import (
	"strings"
	"testing"
	"time"

	"nasmon/internal/app"
	"nasmon/internal/model"
)

func TestRenderInteractiveShowsFanRPMRegular(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 2}
	r := Renderer{Config: cfg}
	rpm := 2200
	s := model.Snapshot{FanRPM: &rpm, MemTotalBytes: 16 << 30, MemUsedBytes: 4 << 30}

	out := r.RenderInteractive(s, 60, 60, DockerSortDefault)
	if !strings.Contains(plainTerminalLine(out), "Fan 2200 rpm") {
		t.Fatalf("regular fan RPM missing")
	}
}

func TestRenderInteractiveShowsFanRPMWithoutBreakingLandscapeWidth(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	rpm := 2200
	s := model.Snapshot{FanRPM: &rpm, MemTotalBytes: 16 << 30, MemUsedBytes: 4 << 30}

	out := r.RenderInteractive(s, 30, 120, DockerSortDefault)
	for _, raw := range strings.Split(out, "\n") {
		line := plainTerminalLine(raw)
		if !strings.Contains(line, "CPU") {
			continue
		}
		if !strings.Contains(line, "Fan 2200 rpm") {
			t.Fatalf("landscape fan RPM missing: %q", line)
		}
		if got := len([]rune(line)); got != 118 {
			t.Fatalf("landscape CPU row width = %d, want 118: %q", got, line)
		}
		return
	}
	t.Fatalf("landscape CPU row not found")
}

func TestRenderInteractiveOmitsFanWhenUnavailable(t *testing.T) {
	r := Renderer{Config: app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 2}}
	out := r.RenderInteractive(model.Snapshot{}, 60, 60, DockerSortDefault)
	if strings.Contains(out, "Fan ") {
		t.Fatalf("fan label rendered without a reading")
	}
}
