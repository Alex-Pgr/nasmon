package collect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaxFanRPM(t *testing.T) {
	root := t.TempDir()
	hwmon := filepath.Join(root, "hwmon0")
	if err := os.MkdirAll(hwmon, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"fan1_input": "1800\n",
		"fan2_input": "2200\n",
		"fan3_input": "bad\n",
	} {
		if err := os.WriteFile(filepath.Join(hwmon, name), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rpm := maxFanRPM(root)
	if rpm == nil || *rpm != 2200 {
		t.Fatalf("maxFanRPM = %v, want 2200", rpm)
	}
}

func TestMaxFanRPMKeepsZeroForStoppedFan(t *testing.T) {
	root := t.TempDir()
	hwmon := filepath.Join(root, "hwmon0")
	if err := os.MkdirAll(hwmon, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hwmon, "fan1_input"), []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rpm := maxFanRPM(root)
	if rpm == nil || *rpm != 0 {
		t.Fatalf("maxFanRPM = %v, want 0", rpm)
	}
}
