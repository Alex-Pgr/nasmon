package collect

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCPUTemperaturePrefersPackageHwmon(t *testing.T) {
	root := t.TempDir()
	hwmon := filepath.Join(root, "hwmon")
	thermal := filepath.Join(root, "thermal")

	writeTestFile(t, filepath.Join(hwmon, "hwmon0", "name"), "acpitz\n")
	writeTestFile(t, filepath.Join(hwmon, "hwmon0", "temp1_input"), "41000\n")
	writeTestFile(t, filepath.Join(hwmon, "hwmon1", "name"), "coretemp\n")
	writeTestFile(t, filepath.Join(hwmon, "hwmon1", "temp1_input"), "57000\n")
	writeTestFile(t, filepath.Join(thermal, "thermal_zone0", "type"), "cpu-thermal\n")
	writeTestFile(t, filepath.Join(thermal, "thermal_zone0", "temp"), "62000\n")

	got := cpuTemperature(hwmon, thermal)
	if got == nil || *got != 57 {
		t.Fatalf("cpuTemperature = %v, want 57", got)
	}
}

func TestCPUTemperatureUsesCPUThermalZoneOnSBC(t *testing.T) {
	root := t.TempDir()
	hwmon := filepath.Join(root, "hwmon")
	thermal := filepath.Join(root, "thermal")

	writeTestFile(t, filepath.Join(hwmon, "hwmon0", "name"), "gpu\n")
	writeTestFile(t, filepath.Join(hwmon, "hwmon0", "temp1_input"), "73000\n")
	writeTestFile(t, filepath.Join(thermal, "thermal_zone0", "type"), "gpu_thermal\n")
	writeTestFile(t, filepath.Join(thermal, "thermal_zone0", "temp"), "71000\n")
	writeTestFile(t, filepath.Join(thermal, "thermal_zone1", "type"), "cpu_thermal\n")
	writeTestFile(t, filepath.Join(thermal, "thermal_zone1", "temp"), "52000\n")

	got := cpuTemperature(hwmon, thermal)
	if got == nil || *got != 52 {
		t.Fatalf("cpuTemperature = %v, want 52", got)
	}
}

func TestCPUTemperatureFallsBackToGenericHwmon(t *testing.T) {
	root := t.TempDir()
	hwmon := filepath.Join(root, "hwmon")
	thermal := filepath.Join(root, "thermal")

	writeTestFile(t, filepath.Join(hwmon, "hwmon0", "name"), "generic\n")
	writeTestFile(t, filepath.Join(hwmon, "hwmon0", "temp1_input"), "48000\n")
	writeTestFile(t, filepath.Join(thermal, "thermal_zone0", "type"), "gpu_thermal\n")
	writeTestFile(t, filepath.Join(thermal, "thermal_zone0", "temp"), "70000\n")

	got := cpuTemperature(hwmon, thermal)
	if got == nil || *got != 48 {
		t.Fatalf("cpuTemperature = %v, want 48", got)
	}
}
