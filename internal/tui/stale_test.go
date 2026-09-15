package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/Alex-Pgr/nas_monitoring/internal/app"
	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

func TestRenderInteractiveShowsStaleDaemonIndicator(t *testing.T) {
	r := Renderer{Config: app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 1}}
	s := model.Snapshot{StateStale: true, StateAge: 17 * time.Second}

	for _, tc := range []struct {
		rows int
		cols int
	}{{60, 80}, {30, 120}} {
		out := r.RenderInteractive(s, tc.rows, tc.cols, DockerSortDefault)
		plain := stripANSI(out)
		if !strings.Contains(plain, "DAEMON STALE 17s") {
			t.Fatalf("stale indicator missing for %dx%d: %q", tc.cols, tc.rows, plain)
		}
	}
}

func TestRenderInteractiveOmitsStaleIndicatorWhenFresh(t *testing.T) {
	r := Renderer{Config: app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 1}}
	out := r.RenderInteractive(model.Snapshot{}, 60, 80, DockerSortDefault)
	if strings.Contains(stripANSI(out), "DAEMON STALE") {
		t.Fatalf("stale indicator rendered for fresh state")
	}
}
