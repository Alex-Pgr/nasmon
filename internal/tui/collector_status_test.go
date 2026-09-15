package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/Alex-Pgr/nas_monitoring/internal/app"
	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

func TestCollectorIssueStates(t *testing.T) {
	now := time.Now()
	interval := 10 * time.Second

	if got := collectorIssue(model.CollectorStatus{}, interval); got != "" {
		t.Fatalf("disabled collector issue = %q", got)
	}
	if got := collectorIssue(model.CollectorStatus{Enabled: true}, interval); got != "" {
		t.Fatalf("pending collector issue = %q", got)
	}
	if got := collectorIssue(model.CollectorStatus{Enabled: true, LastAttempt: now}, interval); got != "UNAVAILABLE" {
		t.Fatalf("unavailable collector issue = %q", got)
	}
	if got := collectorIssue(model.CollectorStatus{Enabled: true, LastAttempt: now, LastSuccess: now.Add(-time.Second)}, interval); got != "ERROR" {
		t.Fatalf("failed latest attempt issue = %q", got)
	}
	staleAt := now.Add(-5 * interval)
	if got := collectorIssue(model.CollectorStatus{Enabled: true, LastAttempt: staleAt, LastSuccess: staleAt}, interval); !strings.HasPrefix(got, "STALE ") {
		t.Fatalf("stale collector issue = %q", got)
	}
}

func TestRenderShowsCollectorProblems(t *testing.T) {
	now := time.Now()
	r := Renderer{Config: app.Config{
		MainInterval:    2 * time.Second,
		DockerInterval:  30 * time.Second,
		SMARTInterval:   time.Hour,
		SystemdInterval: 30 * time.Second,
		MinTermWidth:    36,
		RightMargin:     1,
	}}
	s := model.Snapshot{
		DockerCollector: model.CollectorStatus{Enabled: true, LastAttempt: now},
		SMARTCollector:  model.CollectorStatus{Enabled: true, LastAttempt: now.Add(-5 * time.Hour), LastSuccess: now.Add(-5 * time.Hour)},
	}
	plain := stripANSI(r.RenderInteractive(s, 60, 100, DockerSortDefault))
	if !strings.Contains(plain, "Docker Services [UNAVAILABLE]") {
		t.Fatalf("docker collector warning missing: %q", plain)
	}
	if !strings.Contains(plain, "SMART STALE") {
		t.Fatalf("SMART stale warning missing: %q", plain)
	}
}
