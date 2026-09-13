package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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

func unsetEnv(t *testing.T, name string) {
	t.Helper()
	old, existed := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("Unsetenv(%s): %v", name, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(name, old)
		} else {
			_ = os.Unsetenv(name)
		}
	})
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

func TestLoadConfigReadsFileAndEnvironmentWins(t *testing.T) {
	for _, name := range []string{
		"NAS_INTERFACE",
		"GPU_INFO_HELPER",
		"STORAGE_PATH",
		"NAS_STATE_FILE",
		"DISK_PATHS",
		"MAIN_INTERVAL",
	} {
		unsetEnv(t, name)
	}

	path := filepath.Join(t.TempDir(), "nasmon.env")
	content := "# host config\nNAS_INTERFACE=eno1\nGPU_INFO_HELPER=/opt/gpu-info\nSTORAGE_PATH=/srv/storage\nNAS_STATE_FILE=/tmp/nasmon-state.json\nDISK_PATHS=/,/srv/storage,/mnt/archive\nMAIN_INTERVAL=7\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("NASMON_CONFIG_FILE", path)
	t.Setenv("STORAGE_PATH", "/srv/override")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Interface != "eno1" || cfg.GPUHelper != "/opt/gpu-info" {
		t.Fatalf("host settings not loaded: %+v", cfg)
	}
	if cfg.StoragePath != "/srv/override" {
		t.Fatalf("StoragePath = %q, want environment override", cfg.StoragePath)
	}
	if cfg.StateFile != "/tmp/nasmon-state.json" {
		t.Fatalf("StateFile = %q", cfg.StateFile)
	}
	if cfg.MainInterval != 7*time.Second {
		t.Fatalf("MainInterval = %s", cfg.MainInterval)
	}
	wantPaths := []string{"/", "/srv/storage", "/mnt/archive"}
	if !reflect.DeepEqual(cfg.DiskPaths, wantPaths) {
		t.Fatalf("DiskPaths = %#v, want %#v", cfg.DiskPaths, wantPaths)
	}
}

func TestLoadConfigRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nasmon.env")
	if err := os.WriteFile(path, []byte("NOT_AN_ASSIGNMENT\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("NASMON_CONFIG_FILE", path)
	if _, err := LoadConfig(); err == nil {
		t.Fatalf("LoadConfig accepted malformed config")
	}
}
