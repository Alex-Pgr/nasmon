package doctor

import (
	"bytes"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"nasmon/internal/app"
	"nasmon/internal/model"
	"nasmon/internal/statefile"
)

type fakeInfo struct {
	name string
	dir  bool
	mode os.FileMode
}

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() os.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.dir }
func (f fakeInfo) Sys() any           { return nil }

func missing(path string) error {
	return &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}

func baseProbes(now time.Time) probes {
	return probes{
		goos:   "linux",
		goarch: "arm64",
		stat: func(path string) (os.FileInfo, error) {
			switch path {
			case "/etc/test-nasmon.env":
				return fakeInfo{name: "test-nasmon.env"}, nil
			case "/", "/mnt/data":
				return fakeInfo{name: path, dir: true, mode: os.ModeDir | 0755}, nil
			case "/var/run/docker.sock":
				return fakeInfo{name: "docker.sock", mode: os.ModeSocket}, nil
			case "/usr/local/bin/gpu-helper":
				return fakeInfo{name: "gpu-helper", mode: 0755}, nil
			default:
				return nil, missing(path)
			}
		},
		lookPath: func(command string) (string, error) {
			return "/usr/bin/" + command, nil
		},
		dialUnix: func(string) error { return nil },
		interfaces: func() ([]net.Interface, error) {
			return []net.Interface{{Name: "eth0"}}, nil
		},
		readFile: func(path string) ([]byte, error) {
			if path != "/proc/net/route" {
				return nil, errors.New("unexpected path")
			}
			return []byte("Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\neth0 00000000 0100007F 0003 0 0 100 00000000 0 0 0\n"), nil
		},
		readState: func(string) (statefile.State, error) {
			return statefile.State{WrittenAt: now.Add(-time.Second), Snapshot: model.Snapshot{}}, nil
		},
		now: func() time.Time { return now },
	}
}

func TestRunHealthyARMHost(t *testing.T) {
	t.Setenv("NASMON_CONFIG_FILE", "/etc/test-nasmon.env")
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	cfg := app.Config{
		MainInterval: 2 * time.Second,
		StoragePath:  "/mnt/data",
		DiskPaths:    []string{"/", "/mnt/data"},
		StateFile:    "/run/nasmon/state.json",
		GPUHelper:    "/usr/local/bin/gpu-helper",
	}

	report := run(cfg, baseProbes(now))
	if report.Failed() {
		t.Fatalf("healthy ARM host unexpectedly failed: %+v", report.Checks)
	}
	for _, want := range []string{"linux/arm64", "eth0", "docker.sock reachable", "fresh: 1s old"} {
		found := false
		for _, check := range report.Checks {
			if strings.Contains(check.Detail, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("diagnostic detail %q missing from %+v", want, report.Checks)
		}
	}
}

func TestRunFlagsConfiguredNetworkAndStorageFailures(t *testing.T) {
	t.Setenv("NASMON_CONFIG_FILE", "/etc/test-nasmon.env")
	now := time.Now()
	p := baseProbes(now)
	cfg := app.Config{
		MainInterval: 2 * time.Second,
		StoragePath:  "/missing",
		DiskPaths:    []string{"/"},
		StateFile:    "/run/nasmon/state.json",
		Interface:    "enp99s0",
	}

	report := run(cfg, p)
	if !report.Failed() {
		t.Fatalf("invalid configured host should fail: %+v", report.Checks)
	}
	failures := 0
	for _, check := range report.Checks {
		if check.Level == FAIL {
			failures++
		}
	}
	if failures != 2 {
		t.Fatalf("FAIL count = %d, want 2: %+v", failures, report.Checks)
	}
}

func TestReportWriteSummarizesLevels(t *testing.T) {
	report := Report{Checks: []Check{
		{Level: OK, Name: "platform", Detail: "linux/arm64"},
		{Level: WARN, Name: "Docker", Detail: "not installed"},
		{Level: FAIL, Name: "storage", Detail: "missing"},
	}}
	var b bytes.Buffer
	report.Write(&b)
	out := b.String()
	if !strings.Contains(out, "Summary: 1 OK, 1 WARN, 1 FAIL") {
		t.Fatalf("summary missing: %q", out)
	}
}
