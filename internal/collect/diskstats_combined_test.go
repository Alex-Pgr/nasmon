package collect

import (
	"testing"
	"time"
)

func TestUpdateDiskActivityFromCounters(t *testing.T) {
	diskActivityMu.Lock()
	old := diskActivity
	diskActivity = map[string]diskActivityState{}
	diskActivityMu.Unlock()
	t.Cleanup(func() {
		diskActivityMu.Lock()
		diskActivity = old
		diskActivityMu.Unlock()
	})

	first := time.Unix(100, 0)
	updateDiskActivityFromCounters([]string{"sda"}, map[string]diskSectorCounters{
		"sda": {read: 10, write: 20},
	}, first)

	diskActivityMu.Lock()
	state := diskActivity["sda"]
	diskActivityMu.Unlock()
	if !state.initialized || !state.lastActivity.Equal(first) {
		t.Fatalf("initial activity = %+v, want initialized at %s", state, first)
	}

	unchanged := first.Add(time.Minute)
	updateDiskActivityFromCounters([]string{"sda"}, map[string]diskSectorCounters{
		"sda": {read: 10, write: 20},
	}, unchanged)
	diskActivityMu.Lock()
	state = diskActivity["sda"]
	diskActivityMu.Unlock()
	if !state.lastActivity.Equal(first) {
		t.Fatalf("unchanged counters moved lastActivity to %s", state.lastActivity)
	}

	changed := unchanged.Add(time.Minute)
	updateDiskActivityFromCounters([]string{"sda"}, map[string]diskSectorCounters{
		"sda": {read: 11, write: 20},
	}, changed)
	diskActivityMu.Lock()
	state = diskActivity["sda"]
	diskActivityMu.Unlock()
	if !state.lastActivity.Equal(changed) {
		t.Fatalf("changed counters left lastActivity at %s, want %s", state.lastActivity, changed)
	}
}
