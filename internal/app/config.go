package app

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	MainInterval      time.Duration
	GPUInterval       time.Duration
	DockerInterval    time.Duration
	DiskInterval      time.Duration
	DiskPowerInterval time.Duration
	DiskTempInterval  time.Duration
	DiskQuietWindow   time.Duration
	SMARTInterval     time.Duration
	SystemdInterval   time.Duration
	IPInterval        time.Duration

	Interface    string
	GPUHelper    string
	StoragePath  string
	StateFile    string
	DiskPaths    []string
	RightMargin  int
	MinTermWidth int
	OneShot      bool
	ForceCols    int
	ForceRows    int
}

func envDuration(name string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return time.Duration(n) * time.Second
}

func envInt(name string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	return n
}

func DefaultConfig() Config {
	iface := os.Getenv("NAS_INTERFACE")
	if iface == "" {
		iface = "enp1s0f1"
	}
	helper := os.Getenv("GPU_INFO_HELPER")
	if helper == "" {
		helper = "/usr/local/bin/nas-gpu-info"
	}
	storage := os.Getenv("STORAGE_PATH")
	if storage == "" {
		storage = "/mnt/hdd"
	}
	stateFile := os.Getenv("NAS_STATE_FILE")
	if stateFile == "" {
		stateFile = "/run/nasmon/state.json"
	}

	return Config{
		MainInterval:      envDuration("MAIN_INTERVAL", 2*time.Second),
		GPUInterval:       envDuration("GPU_INTERVAL", 10*time.Second),
		DockerInterval:    envDuration("DOCKER_INTERVAL", 30*time.Second),
		DiskInterval:      envDuration("DISK_LAYOUT_INTERVAL", 15*time.Second),
		DiskPowerInterval: envDuration("DISK_POWER_INTERVAL", 60*time.Second),
		DiskTempInterval:  envDuration("DISK_TEMP_INTERVAL", 15*time.Minute),
		DiskQuietWindow:   envDuration("DISK_QUIET_WINDOW", 5*time.Minute),
		SMARTInterval:     envDuration("SMART_INTERVAL", time.Hour),
		SystemdInterval:   envDuration("SYSTEMD_INTERVAL", 30*time.Second),
		IPInterval:        envDuration("IP_INTERVAL", 60*time.Second),
		Interface:         iface,
		GPUHelper:         helper,
		StoragePath:       storage,
		StateFile:         stateFile,
		DiskPaths:         []string{"/", "/mnt/ssd", "/mnt/hdd"},
		RightMargin:       1,
		MinTermWidth:      36,
		OneShot:           os.Getenv("NAS_MONITOR_ONESHOT") == "1",
		ForceCols:         envInt("NAS_FORCE_COLS"),
		ForceRows:         envInt("NAS_FORCE_ROWS"),
	}
}
