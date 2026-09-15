package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Alex-Pgr/nasmon/internal/app"
	"github.com/Alex-Pgr/nasmon/internal/model"
)

func TestPortraitShortViewportUsesUltraCompact(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, DockerInterval: 30 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	containers := make([]model.Container, 24)
	for i := range containers {
		containers[i] = model.Container{Name: fmt.Sprintf("ctr-%02d", i+1), State: "running", Status: "Up 1h", MemoryBytes: uint64(i+1) << 20}
	}
	s := model.Snapshot{
		CPUUsage:       23,
		MemUsedBytes:   6 << 30,
		MemTotalBytes:  16 << 30,
		SwapUsedBytes:  1 << 30,
		SwapTotalBytes: 12 << 30,
		DiskUsage:      []model.DiskUsage{{Path: "/mnt/fast", UsedBytes: 90 << 30, TotalBytes: 468 << 30}},
		Containers:     containers,
	}

	out := r.RenderInteractiveView(s, 25, 60, DockerSortDefault, 2)
	plain := stripANSI(out)
	for _, want := range []string{"CPU 23%", "RAM 6.0/16.0G", "SWAP 1.0/12.0G", "Docker ↑ 3–22/24 ↓", "ctr-03", "ctr-22"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("portrait compact output missing %q: %q", want, plain)
		}
	}
	for _, unwanted := range []string{"Disk Usage", "Storage Analysis", "ctr-01", "ctr-02", "ctr-23", "ctr-24"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("portrait compact output unexpectedly contains %q: %q", unwanted, plain)
		}
	}
	if strings.Contains(plain, "┌── Health ") {
		t.Fatalf("portrait compact output unexpectedly contains Health panel: %q", plain)
	}
	if got := r.DockerPageSize(s, 25, 60); got != 20 {
		t.Fatalf("portrait compact Docker page size = %d, want 20", got)
	}
	if got := len(strings.Split(strings.TrimSuffix(out, "\n"), "\n")); got != 25 {
		t.Fatalf("portrait compact rendered rows = %d, want 25", got)
	}
}

func TestWideShortViewportKeepsNormalLandscape(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{
		DiskUsage:  []model.DiskUsage{{Path: "/mnt/fast", UsedBytes: 90 << 30, TotalBytes: 468 << 30}},
		Containers: []model.Container{{Name: "one", State: "running", Status: "Up 1h"}},
	}

	plain := stripANSI(r.RenderInteractiveView(s, 25, 120, DockerSortDefault, 0))
	if !strings.Contains(plain, "Disk Usage") || !strings.Contains(plain, "Health") {
		t.Fatalf("wide short viewport should keep normal landscape: %q", plain)
	}
}

func TestTallPortraitKeepsRegularLayout(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}
	s := model.Snapshot{
		DiskUsage:  []model.DiskUsage{{Path: "/mnt/fast", UsedBytes: 90 << 30, TotalBytes: 468 << 30}},
		Containers: []model.Container{{Name: "one", State: "running", Status: "Up 1h"}},
	}

	plain := stripANSI(r.RenderInteractiveView(s, 40, 60, DockerSortDefault, 0))
	if !strings.Contains(plain, "Disk Usage") || !strings.Contains(plain, "Storage Analysis") {
		t.Fatalf("tall portrait should keep regular layout: %q", plain)
	}
}
