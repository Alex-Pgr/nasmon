package tui

import (
	"strings"
	"testing"
	"time"

	"nasmon/internal/app"
	"nasmon/internal/model"
)

func TestUnicodeUserTextStaysWithinLandscapeWidth(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{
		DiskUsage: []model.DiskUsage{{Path: "/数据", UsedBytes: 4 << 30, TotalBytes: 10 << 30, Percent: 40}},
		DiskHealth: []model.DiskHealth{{Device: "nvme0n1", Brand: "磁盘", Temperature: "40°C", Health: "OK"}},
		Containers: []model.Container{{Name: "服务🙂", State: "running", Status: "Up 1h", MemoryBytes: 64 << 20}},
	}
	out := r.RenderInteractive(s, 30, 120, DockerSortDefault)
	_, drawWidth, landscape := effectiveLayout(r, 30, 120)
	if !landscape {
		t.Fatal("expected landscape layout")
	}
	if !strings.Contains(out, "服务🙂") || !strings.Contains(out, "/数据") || !strings.Contains(out, "磁盘") {
		t.Fatalf("Unicode content missing from render: %q", stripANSI(out))
	}
	for n, raw := range strings.Split(out, "\n") {
		if got := cellWidth(raw); got > drawWidth {
			t.Fatalf("line %d width = %d, draw width = %d: %q", n+1, got, drawWidth, stripANSI(raw))
		}
	}
}

func TestCompactLandscapeHeaderIsComposedDirectly(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{}
	for i := 0; i < 20; i++ {
		s.Containers = append(s.Containers, model.Container{Name: "service", State: "running", Status: "Up 1h"})
	}
	out := r.RenderInteractive(s, 18, 120, DockerSortDefault)
	lines := strings.Split(out, "\n")
	if len(lines) == 0 || !strings.Contains(stripANSI(lines[0]), "NAS Health Monitor") {
		t.Fatalf("compact title missing: %q", stripANSI(out))
	}
	if strings.Contains(stripANSI(lines[0]), "══") {
		t.Fatalf("compact mode unexpectedly rendered full header: %q", stripANSI(lines[0]))
	}
}
