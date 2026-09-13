package app

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultConfigFile = "/etc/nasmon/nasmon.env"

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

type configLookup func(string) (string, bool)

func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=value", path, lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNo)
		}
		if len(value) >= 2 {
			first, last := value[0], value[len(value)-1]
			if (first == '\'' && last == '\'') || (first == '"' && last == '"') {
				value = value[1 : len(value)-1]
			}
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func combinedLookup(fileValues map[string]string) configLookup {
	return func(name string) (string, bool) {
		if value, ok := os.LookupEnv(name); ok {
			return strings.TrimSpace(value), true
		}
		value, ok := fileValues[name]
		return strings.TrimSpace(value), ok
	}
}

func envDuration(lookup configLookup, name string, def time.Duration) time.Duration {
	v, _ := lookup(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return time.Duration(n) * time.Second
}

func envInt(lookup configLookup, name string) int {
	v, _ := lookup(name)
	n, _ := strconv.Atoi(v)
	return n
}

func envList(lookup configLookup, name string, def []string) []string {
	raw, _ := lookup(name)
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

func configFromLookup(lookup configLookup) Config {
	storage, _ := lookup("STORAGE_PATH")
	if storage == "" {
		storage = "/"
	}
	stateFile, _ := lookup("NAS_STATE_FILE")
	if stateFile == "" {
		stateFile = "/run/nasmon/state.json"
	}
	iface, _ := lookup("NAS_INTERFACE")
	helper, _ := lookup("GPU_INFO_HELPER")
	oneShot, _ := lookup("NAS_MONITOR_ONESHOT")

	return Config{
		MainInterval:      envDuration(lookup, "MAIN_INTERVAL", 2*time.Second),
		GPUInterval:       envDuration(lookup, "GPU_INTERVAL", 10*time.Second),
		DockerInterval:    envDuration(lookup, "DOCKER_INTERVAL", 30*time.Second),
		DiskInterval:      envDuration(lookup, "DISK_LAYOUT_INTERVAL", 15*time.Second),
		DiskPowerInterval: envDuration(lookup, "DISK_POWER_INTERVAL", 60*time.Second),
		DiskTempInterval:  envDuration(lookup, "DISK_TEMP_INTERVAL", 15*time.Minute),
		DiskQuietWindow:   envDuration(lookup, "DISK_QUIET_WINDOW", 5*time.Minute),
		SMARTInterval:     envDuration(lookup, "SMART_INTERVAL", time.Hour),
		SystemdInterval:   envDuration(lookup, "SYSTEMD_INTERVAL", 30*time.Second),
		IPInterval:        envDuration(lookup, "IP_INTERVAL", 60*time.Second),
		Interface:         iface,
		GPUHelper:         helper,
		StoragePath:       storage,
		StateFile:         stateFile,
		DiskPaths:         envList(lookup, "DISK_PATHS", []string{"/"}),
		RightMargin:       1,
		MinTermWidth:      36,
		OneShot:           oneShot == "1",
		ForceCols:         envInt(lookup, "NAS_FORCE_COLS"),
		ForceRows:         envInt(lookup, "NAS_FORCE_ROWS"),
	}
}

// DefaultConfig reads only the current process environment. It is useful for
// tests and embedded use; command binaries use LoadConfig so daemon and client
// share the same host config file.
func DefaultConfig() Config {
	return configFromLookup(combinedLookup(nil))
}

// LoadConfig loads the optional host config file and then overlays process
// environment variables. This keeps nasmond and nasmon on the same state path,
// intervals and host settings even when the client is launched from a shell.
func LoadConfig() (Config, error) {
	path := strings.TrimSpace(os.Getenv("NASMON_CONFIG_FILE"))
	if path == "" {
		path = DefaultConfigFile
	}
	values, err := parseEnvFile(path)
	if err != nil {
		return Config{}, err
	}
	return configFromLookup(combinedLookup(values)), nil
}
