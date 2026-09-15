package doctor

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Alex-Pgr/nasmon/internal/app"
	"github.com/Alex-Pgr/nasmon/internal/statefile"
)

type Level string

const (
	OK   Level = "OK"
	WARN Level = "WARN"
	FAIL Level = "FAIL"
)

type Check struct {
	Level  Level
	Name   string
	Detail string
}

type Report struct {
	Checks []Check
}

func (r Report) Failed() bool {
	for _, check := range r.Checks {
		if check.Level == FAIL {
			return true
		}
	}
	return false
}

func (r Report) Write(w io.Writer) {
	for _, check := range r.Checks {
		fmt.Fprintf(w, "%-4s %-16s %s\n", check.Level, check.Name, check.Detail)
	}
	ok, warn, fail := 0, 0, 0
	for _, check := range r.Checks {
		switch check.Level {
		case OK:
			ok++
		case WARN:
			warn++
		case FAIL:
			fail++
		}
	}
	fmt.Fprintf(w, "\nSummary: %d OK, %d WARN, %d FAIL\n", ok, warn, fail)
}

type probes struct {
	goos       string
	goarch     string
	stat       func(string) (os.FileInfo, error)
	lookPath   func(string) (string, error)
	dialUnix   func(string) error
	interfaces func() ([]net.Interface, error)
	readFile   func(string) ([]byte, error)
	readState  func(string) (statefile.State, error)
	now        func() time.Time
}

func defaultProbes() probes {
	return probes{
		goos:     runtime.GOOS,
		goarch:   runtime.GOARCH,
		stat:     os.Stat,
		lookPath: exec.LookPath,
		dialUnix: func(path string) error {
			conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
			if err == nil {
				conn.Close()
			}
			return err
		},
		interfaces: net.Interfaces,
		readFile:   os.ReadFile,
		readState:  statefile.ReadState,
		now:        time.Now,
	}
}

func Run(cfg app.Config) Report {
	return run(cfg, defaultProbes())
}

func run(cfg app.Config, p probes) Report {
	var r Report
	add := func(level Level, name, detail string) {
		r.Checks = append(r.Checks, Check{Level: level, Name: name, Detail: detail})
	}

	if p.goos != "linux" {
		add(FAIL, "platform", fmt.Sprintf("%s/%s; nasmon currently requires Linux", p.goos, p.goarch))
	} else {
		add(OK, "platform", p.goos+"/"+p.goarch)
	}

	configPath := app.ConfigFilePath()
	if _, err := p.stat(configPath); err == nil {
		add(OK, "config", configPath)
	} else if os.IsNotExist(err) {
		add(WARN, "config", configPath+" not found; using defaults/environment")
	} else {
		add(WARN, "config", fmt.Sprintf("cannot stat %s: %v", configPath, err))
	}

	checkDir := func(name, path string) {
		info, err := p.stat(path)
		switch {
		case err != nil:
			add(FAIL, name, fmt.Sprintf("%s: %v", path, err))
		case !info.IsDir():
			add(FAIL, name, path+" is not a directory")
		default:
			add(OK, name, path)
		}
	}
	checkDir("storage", cfg.StoragePath)
	for _, path := range cfg.DiskPaths {
		checkDir("disk path", path)
	}

	iface := cfg.Interface
	if iface == "" {
		iface = defaultRouteInterface(p.readFile)
	}
	if iface == "" {
		add(WARN, "network", "no configured or default-route interface found")
	} else if interfacePresent(iface, p.interfaces) {
		add(OK, "network", iface)
	} else if cfg.Interface != "" {
		add(FAIL, "network", "configured interface "+iface+" not found")
	} else {
		add(WARN, "network", "default-route interface "+iface+" not found")
	}

	for _, command := range []string{"smartctl", "hdparm", "systemctl"} {
		if path, err := p.lookPath(command); err == nil {
			add(OK, command, path)
		} else {
			add(WARN, command, "not found in PATH; related metrics will be unavailable")
		}
	}

	if cfg.GPUHelper == "" {
		add(OK, "GPU helper", "disabled")
	} else if info, err := p.stat(cfg.GPUHelper); err != nil {
		add(WARN, "GPU helper", fmt.Sprintf("%s: %v", cfg.GPUHelper, err))
	} else if info.Mode()&0111 == 0 {
		add(WARN, "GPU helper", cfg.GPUHelper+" is not executable")
	} else {
		add(OK, "GPU helper", cfg.GPUHelper)
	}

	const dockerSocket = "/var/run/docker.sock"
	if _, err := p.stat(dockerSocket); os.IsNotExist(err) {
		add(WARN, "Docker", dockerSocket+" not found")
	} else if err != nil {
		add(WARN, "Docker", fmt.Sprintf("cannot stat %s: %v", dockerSocket, err))
	} else if err := p.dialUnix(dockerSocket); err != nil {
		add(WARN, "Docker", fmt.Sprintf("cannot connect to %s: %v", dockerSocket, err))
	} else {
		add(OK, "Docker", dockerSocket+" reachable")
	}

	state, err := p.readState(cfg.StateFile)
	if err != nil {
		if os.IsNotExist(err) {
			add(WARN, "state", cfg.StateFile+" not found; is nasmond running?")
		} else {
			add(WARN, "state", fmt.Sprintf("cannot read %s: %v", cfg.StateFile, err))
		}
	} else if state.WrittenAt.IsZero() {
		add(WARN, "state", cfg.StateFile+" has no written_at timestamp")
	} else {
		age := p.now().Sub(state.WrittenAt)
		if age < 0 {
			age = 0
		}
		if age > 4*cfg.MainInterval {
			add(WARN, "state", fmt.Sprintf("stale: %s old", compactDuration(age)))
		} else {
			add(OK, "state", fmt.Sprintf("fresh: %s old", compactDuration(age)))
		}
	}

	return r
}

func defaultRouteInterface(readFile func(string) ([]byte, error)) string {
	data, err := readFile("/proc/net/route")
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	if scanner.Scan() {
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == "00000000" {
			return fields[0]
		}
	}
	return ""
}

func interfacePresent(name string, list func() ([]net.Interface, error)) bool {
	interfaces, err := list()
	if err != nil {
		return false
	}
	for _, iface := range interfaces {
		if iface.Name == name {
			return true
		}
	}
	return false
}

func compactDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}
