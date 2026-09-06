package tui

import (
	"strings"
	"testing"

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
	plain := plainTerminalLine(frame)
	nameX := runeIndex(plain, "NAMES") + 1
	ramX := runeIndex(plain, "RAM") + 1
	statusX := runeIndex(plain, "STATUS") + 1

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

func TestParseSGRMouse(t *testing.T) {
	ev, ok := parseSGRMouse("0;12;7")
	if !ok || ev.X != 12 || ev.Y != 7 {
		t.Fatalf("left click = %+v, ok=%v", ev, ok)
	}
	if _, ok := parseSGRMouse("64;12;7"); ok {
		t.Fatalf("wheel event must be ignored")
	}
	if _, ok := parseSGRMouse("1;12;7"); ok {
		t.Fatalf("non-left button must be ignored")
	}
}
