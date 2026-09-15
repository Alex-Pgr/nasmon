package main

import (
	"testing"
	"time"

	"github.com/Alex-Pgr/nasmon/internal/model"
)

func TestApplyStateFreshness(t *testing.T) {
	interval := 2 * time.Second

	fresh := model.Snapshot{}
	applyStateFreshness(&fresh, time.Now().Add(-3*interval), interval)
	if fresh.StateStale {
		t.Fatalf("state marked stale before 4x interval")
	}
	if fresh.StateAge < 5*time.Second || fresh.StateAge > 7*time.Second {
		t.Fatalf("unexpected fresh state age: %s", fresh.StateAge)
	}

	stale := model.Snapshot{}
	applyStateFreshness(&stale, time.Now().Add(-5*interval), interval)
	if !stale.StateStale {
		t.Fatalf("state not marked stale after 4x interval")
	}

	missing := model.Snapshot{}
	applyStateFreshness(&missing, time.Time{}, interval)
	if !missing.StateStale {
		t.Fatalf("missing written_at must be stale")
	}
}
