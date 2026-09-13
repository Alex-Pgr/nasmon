package tui

import (
	"strings"
	"testing"

	"nasmon/internal/app"
	"nasmon/internal/model"
)

func TestSortDockerContainersModes(t *testing.T) {
	original := []model.Container{
		{ID: "1", Name: "zeta", MemoryBytes: 200},
		{ID: "2", Name: "alpha", MemoryBytes: 900},
		{ID: "3", Name: "middle", MemoryBytes: 500},
	}
	selected := []model.Container{original[1], original[2], original[0]}

	got := sortDockerContainers(selected, original, DockerSortDefault)
	if got[0].Name != "zeta" || got[1].Name != "alpha" || got[2].Name != "middle" {
		t.Fatalf("default order = %q, %q, %q", got[0].Name, got[1].Name, got[2].Name)
	}
	got = sortDockerContainers(selected, original, DockerSortNameAsc)
	if got[0].Name != "alpha" || got[1].Name != "middle" || got[2].Name != "zeta" {
		t.Fatalf("name asc order = %q, %q, %q", got[0].Name, got[1].Name, got[2].Name)
	}
	got = sortDockerContainers(selected, original, DockerSortRAMDesc)
	if got[0].Name != "alpha" || got[1].Name != "middle" || got[2].Name != "zeta" {
		t.Fatalf("RAM desc order = %q, %q, %q", got[0].Name, got[1].Name, got[2].Name)
	}
}

func TestDockerSortForClick(t *testing.T) {
	frame := "\033[2K\r│   NAMES       RAM  STATUS\n"
	nameX := cellIndex(frame, "NAMES") + 1
	ramX := cellIndex(frame, "RAM") + 1
	statusX := cellIndex(frame, "STATUS") + 1

	mode, changed := DockerSortForClick(frame, nameX, 1, DockerSortDefault)
	if !changed || mode != DockerSortNameAsc {
		t.Fatalf("NAMES first click = %v, changed=%v", mode, changed)
	}
	mode, changed = DockerSortForClick(frame, nameX, 1, mode)
	if !changed || mode != DockerSortNameDesc {
		t.Fatalf("NAMES second click = %v, changed=%v", mode, changed)
	}
	mode, changed = DockerSortForClick(frame, ramX, 1, DockerSortDefault)
	if !changed || mode != DockerSortRAMDesc {
		t.Fatalf("RAM first click = %v, changed=%v", mode, changed)
	}
	mode, changed = DockerSortForClick(frame, statusX, 1, mode)
	if !changed || mode != DockerSortDefault {
		t.Fatalf("STATUS click = %v, changed=%v", mode, changed)
	}
}

func TestRenderInteractiveShowsSortArrow(t *testing.T) {
	r := Renderer{}
	s := model.Snapshot{Containers: []model.Container{{Name: "a", State: "running", Status: "Up 1h", MemoryBytes: 10}}}
	out := r.RenderInteractive(s, 60, 60, DockerSortRAMDesc)
	if !strings.Contains(out, "RAM↓") {
		t.Fatalf("RAM sort arrow missing")
	}
}

func TestParseSGRMouseAndWheel(t *testing.T) {
	ev, ok := parseSGRMouse("0;12;7")
	if !ok || ev.X != 12 || ev.Y != 7 {
		t.Fatalf("left click = %+v, ok=%v", ev, ok)
	}
	if _, ok := parseSGRMouse("64;12;7"); ok {
		t.Fatalf("wheel must not be reported as a click")
	}
	wheel, ok := parseSGRInput("64;12;7")
	if !ok || wheel.Kind != InputUp {
		t.Fatalf("wheel up = %+v, ok=%v", wheel, ok)
	}
	wheel, ok = parseSGRInput("65;12;7")
	if !ok || wheel.Kind != InputDown {
		t.Fatalf("wheel down = %+v, ok=%v", wheel, ok)
	}
	if _, ok := parseSGRMouse("1;12;7"); ok {
		t.Fatalf("non-left button must be ignored")
	}
}

func TestClampDockerOffset(t *testing.T) {
	if got := ClampDockerOffset(99, 20, 5); got != 15 {
		t.Fatalf("offset = %d, want 15", got)
	}
	if got := ClampDockerOffset(-3, 20, 5); got != 0 {
		t.Fatalf("negative offset = %d", got)
	}
	if got := ClampDockerOffset(4, 4, 4); got != 0 {
		t.Fatalf("full-page offset = %d", got)
	}
}

func TestLandscapeDockerViewportSortsBeforeSlicing(t *testing.T) {
	r := Renderer{Config: app.Config{MainInterval: 2, MinTermWidth: 80, RightMargin: 1}}
	s := model.Snapshot{Containers: []model.Container{
		{Name: "c", MemoryBytes: 300},
		{Name: "a", MemoryBytes: 100},
		{Name: "b", MemoryBytes: 200},
	}}
	out := r.RenderInteractiveView(s, 10, 100, DockerSortNameAsc, 1)
	if strings.Contains(out, " a ") || !strings.Contains(out, "b") || !strings.Contains(out, "c") {
		t.Fatalf("viewport did not slice the globally sorted list: %q", stripANSI(out))
	}
	if !strings.Contains(stripANSI(out), "2–3/3") {
		t.Fatalf("viewport range indicator missing: %q", stripANSI(out))
	}
}
