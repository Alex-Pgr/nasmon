package collect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadGPUUsageSkipsInvalidPath(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad")
	good := filepath.Join(dir, "good")
	if err := os.WriteFile(bad, []byte("not-a-number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, []byte("73\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := readGPUUsage([]string{bad, good})
	if got == nil || *got != 73 {
		t.Fatalf("readGPUUsage() = %v, want 73", got)
	}
}

func TestReadGPUUsageClampsPercent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "busy")
	if err := os.WriteFile(path, []byte("137\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := readGPUUsage([]string{path})
	if got == nil || *got != 100 {
		t.Fatalf("readGPUUsage() = %v, want 100", got)
	}
}

func TestReadGPUUsageUnavailable(t *testing.T) {
	if got := readGPUUsage(nil); got != nil {
		t.Fatalf("readGPUUsage(nil) = %v, want nil", *got)
	}
}
