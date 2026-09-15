package statefile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Alex-Pgr/nasmon/internal/model"
)

func TestWriteAtomicReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "state.json")
	cpuTemp := 48.0
	gpu := 37
	rpm := 2200
	want := model.Snapshot{
		UpdatedAt:      time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
		StartedAt:      time.Date(2026, 9, 12, 20, 0, 0, 0, time.UTC),
		Uptime:         4*time.Hour + 12*time.Minute,
		CPUUsage:       42,
		CPUTempC:       &cpuTemp,
		FanRPM:         &rpm,
		GPUUsage:       &gpu,
		GPUVCN:         "IDLE",
		MemUsedBytes:   5 << 30,
		MemTotalBytes:  16 << 30,
		MemPercent:     31,
		StoragePath:    "/mnt/fast",
		StoragePercent: 26,
		FailedUnits:    1,
		Containers:     []model.Container{{ID: "1", Name: "paperless", State: "running", MemoryBytes: 512 << 20}},
		DiskUsage:      []model.DiskUsage{{Path: "/", UsedBytes: 9 << 30, TotalBytes: 40 << 30, Percent: 23}},
		DiskHealth:     []model.DiskHealth{{Device: "/dev/nvme0n1", Temperature: "48°C", Health: "OK"}},
	}

	if err := WriteAtomic(path, want); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	state, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if state.WrittenAt.IsZero() {
		t.Fatalf("ReadState lost written_at metadata")
	}
	if !reflect.DeepEqual(state.Snapshot, want) {
		t.Fatalf("round trip mismatch\ngot:  %#v\nwant: %#v", state.Snapshot, want)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("compat Read mismatch\ngot:  %#v\nwant: %#v", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat state: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("state mode = %o, want 600", perm)
	}
	info, err = os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat state directory: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Fatalf("state directory mode = %o, want 700", perm)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw state: %v", err)
	}
	var envelope struct {
		Version   int       `json:"version"`
		WrittenAt time.Time `json:"written_at"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("decode raw state: %v", err)
	}
	if envelope.Version != Version {
		t.Fatalf("version = %d, want %d", envelope.Version, Version)
	}
	if envelope.WrittenAt.IsZero() {
		t.Fatalf("written_at is zero")
	}

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".state-*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary state files left behind: %v", matches)
	}
}

func TestWriteAtomicDoesNotPersistClientFreshness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	snapshot := model.Snapshot{CPUUsage: 42, StateStale: true, StateAge: time.Hour}
	if err := WriteAtomic(path, snapshot); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw state: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "StateStale") || strings.Contains(text, "StateAge") {
		t.Fatalf("client-only freshness fields were persisted: %s", text)
	}
	state, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if state.Snapshot.StateStale || state.Snapshot.StateAge != 0 {
		t.Fatalf("client-only freshness metadata survived round trip: %+v", state.Snapshot)
	}
}

func TestWriteAtomicReplacesExistingSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := WriteAtomic(path, model.Snapshot{CPUUsage: 10}); err != nil {
		t.Fatalf("first WriteAtomic: %v", err)
	}
	if err := WriteAtomic(path, model.Snapshot{CPUUsage: 77}); err != nil {
		t.Fatalf("second WriteAtomic: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.CPUUsage != 77 {
		t.Fatalf("CPUUsage = %d, want replacement value 77", got.CPUUsage)
	}
}

func TestReadRejectsUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"version":999,"snapshot":{}}`), 0600); err != nil {
		t.Fatalf("write test state: %v", err)
	}
	_, err := Read(path)
	if err == nil {
		t.Fatalf("Read accepted unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported state version 999") {
		t.Fatalf("unexpected version error: %v", err)
	}
}

func TestAcquireWriterLockIsExclusiveAndReleasable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime", "state.json")
	first, err := AcquireWriterLock(path)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	defer first.Close()

	info, err := os.Stat(filepath.Join(filepath.Dir(path), "nasmond.lock"))
	if err != nil {
		t.Fatalf("stat lock: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("lock mode = %o, want 600", perm)
	}

	second, err := AcquireWriterLock(path)
	if err == nil {
		second.Close()
		t.Fatalf("second writer lock unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "another nasmond") {
		t.Fatalf("unexpected lock error: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first lock: %v", err)
	}
	third, err := AcquireWriterLock(path)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	third.Close()
}
