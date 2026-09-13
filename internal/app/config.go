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

func envList(name string, def []string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return append([]string(nil), def...)
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), def...)
	}
	return out
}

func DefaultConfig() Config {
	storage := strings.TrimSpace(os.Getenv("STORAGE_PATH"))
	if storage == "" {
		storage = "/"
	}
	stateFile := strings.TrimSpace(os.Getenv("NAS_STATE_FILE"))
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
		Interface:         strings.TrimSpace(os.Getenv("NAS_INTERFACE")),
		GPUHelper:         strings.TrimSpace(os.Getenv("GPU_INFO_HELPER")),
		StoragePath:       storage,
		StateFile:         stateFile,
		DiskPaths:         envList("DISK_PATHS", []string{"/"}),
		RightMargin:       1,
		MinTermWidth:      36,
		OneShot:           os.Getenv("NAS_MONITOR_ONESHOT") == "1",
		ForceCols:         envInt("NAS_FORCE_COLS"),
		ForceRows:         envInt("NAS_FORCE_ROWS"),
	}
}
