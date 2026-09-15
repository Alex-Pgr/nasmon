package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Alex-Pgr/nasmon/internal/app"
	"github.com/Alex-Pgr/nasmon/internal/model"
)

func TestRegularPortraitDockerViewportScrolls(t *testing.T) {
	cfg := app.Config{MainInterval: 2 * time.Second, DockerInterval: 30 * time.Second, MinTermWidth: 80, RightMargin: 2}
	r := Renderer{Config: cfg}

	containers := make([]model.Container, 12)
	for i := range containers {
		containers[i] = model.Container{Name: fmt.Sprintf("ctr-%02d", i+1), State: "running", Status: "Up 1h"}
	}
	s := model.Snapshot{Containers: containers}

	rows := 0
	page := 0
	for candidate := mobileCompactMaxRows + 1; candidate < 100; candidate++ {
		p := r.DockerPageSize(s, candidate, 79)
		if p > 0 && p < len(containers) {
			rows, page = candidate, p
			break
		}
	}
	if rows == 0 {
		t.Fatal("could not find a regular portrait height with a partial Docker viewport")
	}

	out := stripANSI(r.RenderInteractiveView(s, rows, 79, DockerSortDefault, 1))
	wantTitle := fmt.Sprintf("Docker Services ↑ 2–%d/%d", page+1, len(containers))
	if !strings.Contains(out, wantTitle) {
		t.Fatalf("regular Docker viewport title missing %q: %q", wantTitle, out)
	}
	if strings.Contains(out, "ctr-01") {
		t.Fatalf("regular Docker viewport did not apply offset: %q", out)
	}
	if !strings.Contains(out, "ctr-02") {
		t.Fatalf("regular Docker viewport missing first scrolled container: %q", out)
	}
}
