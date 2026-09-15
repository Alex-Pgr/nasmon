package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Alex-Pgr/nas_monitoring/internal/app"
	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

func TestSelectDockerContainersPrioritizesRestartsThenRAM(t *testing.T) {
	in := []model.Container{
		{Name: "small", MemoryBytes: 100},
		{Name: "big", MemoryBytes: 900},
		{Name: "restarted", Restarts: 1, MemoryBytes: 1},
		{Name: "medium", MemoryBytes: 500},
	}
	got := selectDockerContainers(in, 3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Name != "restarted" || got[1].Name != "big" || got[2].Name != "medium" {
		t.Fatalf("unexpected order: %q, %q, %q", got[0].Name, got[1].Name, got[2].Name)
	}
}

func TestInteractiveRegularFitsRowsAndUsesCompactHeader(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{
		ZRAMTotalBytes: 3 << 30,
		DiskUsage:      make([]model.DiskUsage, 5),
		DiskHealth:     make([]model.DiskHealth, 3),
		StoragePath:    "/mnt/hdd",
	}
	for i := 0; i < 10; i++ {
		s.Containers = append(s.Containers, model.Container{
			Name:        fmt.Sprintf("service-%02d", i),
			Status:      "Up 1 hour (healthy)",
			State:       "running",
			Health:      "healthy",
			MemoryBytes: uint64(i+1) * 100 << 20,
		})
	}
	s.Containers[0].Restarts = 2

	const rows = 44
	out := r.RenderInteractive(s, rows, 58, DockerSortDefault)
	if got := strings.Count(out, "\n"); got > rows {
		t.Fatalf("rendered %d rows, terminal has %d", got, rows)
	}
	if !strings.Contains(out, "NAS Health Monitor 2s") {
		t.Fatalf("compact header missing")
	}
	if strings.Contains(out, "Обновление:") {
		t.Fatalf("full header unexpectedly rendered")
	}
	if strings.Contains(out, "(healthy)") {
		t.Fatalf("healthy suffix should not be rendered")
	}
	if !strings.Contains(out, "RAM") {
		t.Fatalf("RAM column missing")
	}
	if !strings.Contains(out, "service-00") {
		t.Fatalf("restarted service must survive compact selection")
	}
}
