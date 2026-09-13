package app

import (
	"reflect"
	"testing"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"NAS_INTERFACE",
		"GPU_INFO_HELPER",
		"STORAGE_PATH",
		"NAS_STATE_FILE",
		"DISK_PATHS",
	} {
		t.Setenv(name, "")
	}
}

func TestDefaultConfigUsesPortableHostDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg := DefaultConfig()
	if cfg.Interface != "" {
		t.Fatalf("Interface = %q, want autodetect", cfg.Interface)
	}
	if cfg.GPUHelper != "" {
		t.Fatalf("GPUHelper = %q, want disabled", cfg.GPUHelper)
	}
	if cfg.StoragePath != "/" {
		t.Fatalf("StoragePath = %q, want /", cfg.StoragePath)
	}
	if !reflect.DeepEqual(cfg.DiskPaths, []string{"/"}) {
		t.Fatalf("DiskPaths = %#v, want only /", cfg.DiskPaths)
	}
	if cfg.StateFile != "/run/nasmon/state.json" {
		t.Fatalf("StateFile = %q", cfg.StateFile)
	}
}

func TestDefaultConfigReadsHostOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("NAS_INTERFACE", " eno1 ")
	t.Setenv("GPU_INFO_HELPER", " /opt/bin/gpu-info ")
	t.Setenv("STORAGE_PATH", " /srv/storage ")
	t.Setenv("DISK_PATHS", " /, /srv/storage, , /mnt/archive ")

	cfg := DefaultConfig()
	if cfg.Interface != "eno1" {
		t.Fatalf("Interface = %q", cfg.Interface)
	}
	if cfg.GPUHelper != "/opt/bin/gpu-info" {
		t.Fatalf("GPUHelper = %q", cfg.GPUHelper)
	}
	if cfg.StoragePath != "/srv/storage" {
		t.Fatalf("StoragePath = %q", cfg.StoragePath)
	}
	wantPaths := []string{"/", "/srv/storage", "/mnt/archive"}
	if !reflect.DeepEqual(cfg.DiskPaths, wantPaths) {
		t.Fatalf("DiskPaths = %#v, want %#v", cfg.DiskPaths, wantPaths)
	}
}
