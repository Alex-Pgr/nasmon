package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type CPUStat struct{ Total, Idle uint64 }
type NetStat struct{ RX, TX uint64 }

type Snapshot struct {
	Time                 time.Time
	Uptime               string
	CPUPercent           float64
	IOWaitPercent        float64
	CPUTemp              string
	GPUTemp              string
	VCN                  string
	MemUsed, MemTotal    uint64
	SwapUsed, SwapTotal  uint64
	Load1, Load5, Load15 string
	Interface, IP        string
	RXPerSec, TXPerSec   float64
	Disks                []DiskUsage
	Docker               []DockerService
}

type DiskUsage struct{ Path, FS, Size, Used, Avail, Percent string }
type DockerService struct{ Name, Status string }

type State struct {
	prevCPU    CPUStat
	prevIOWait uint64
	prevNet    NetStat
	prevAt     time.Time
}

func main() {
	interval := flag.Duration("interval", 5*time.Second, "metrics refresh interval")
	iface := flag.String("interface", envOr("NAS_INTERFACE", "enp1s0f1"), "network interface")
	storage := flag.String("storage", envOr("STORAGE_PATH", "/mnt/hdd"), "main storage path")
	flag.Parse()
	if *interval < time.Second {
		*interval = time.Second
	}
	if *interval > time.Hour {
		*interval = time.Hour
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st := &State{}
	hideCursor()
	defer func() { showCursor(); fmt.Print("\x1b[0m\n") }()

	for {
		snap := collect(ctx, st, *iface)
		render(snap, *storage, *interval)
		select {
		case <-ctx.Done():
			return
		case <-time.After(*interval):
		}
	}
}

func collect(ctx context.Context, st *State, requestedIface string) Snapshot {
	now := time.Now()
	s := Snapshot{Time: now, VCN: "N/A"}
	s.Uptime = readUptime()
	cpu, iowait := readCPU()
	if st.prevCPU.Total > 0 && cpu.Total > st.prevCPU.Total {
		dt := float64(cpu.Total - st.prevCPU.Total)
		idle := float64(cpu.Idle - st.prevCPU.Idle)
		s.CPUPercent = 100 * (dt - idle) / dt
		if iowait >= st.prevIOWait {
			s.IOWaitPercent = 100 * float64(iowait-st.prevIOWait) / dt
		}
	}
	st.prevCPU, st.prevIOWait = cpu, iowait
	s.CPUTemp = readCPUTemp()
	s.GPUTemp, s.VCN = readGPU(ctx)
	s.MemUsed, s.MemTotal, s.SwapUsed, s.SwapTotal = readMem()
	s.Load1, s.Load5, s.Load15 = readLoad()
	s.Interface = detectInterface(requestedIface)
	s.IP = readIP(ctx, s.Interface)
	net := readNet(s.Interface)
	if !st.prevAt.IsZero() {
		sec := now.Sub(st.prevAt).Seconds()
		if sec > 0 && net.RX >= st.prevNet.RX && net.TX >= st.prevNet.TX {
			s.RXPerSec = float64(net.RX-st.prevNet.RX) / sec
			s.TXPerSec = float64(net.TX-st.prevNet.TX) / sec
		}
	}
	st.prevNet, st.prevAt = net, now
	s.Disks = readDisks(ctx, []string{"/", "/mnt/ssd", "/mnt/hdd"})
	s.Docker = readDocker(ctx)
	return s
}

func render(s Snapshot, storage string, interval time.Duration) {
	width := terminalWidth()
	if width < 36 {
		width = 36
	}
	clearScreen()
	title := fmt.Sprintf(" NAS Health Monitor  •  %s  •  refresh %s ", s.Time.Format("15:04:05"), interval)
	fmt.Println(cyan(fit(title, width)))
	fmt.Println(dim(strings.Repeat("─", width)))

	fmt.Println(bold("System"))
	fmt.Printf("Uptime: %-24s CPU: %5.1f%%  Temp: %s\n", s.Uptime, s.CPUPercent, s.CPUTemp)
	fmt.Printf("Load:   %s %s %s          iowait: %4.1f%%\n", s.Load1, s.Load5, s.Load15, s.IOWaitPercent)
	fmt.Printf("RAM:    %s  %s / %s\n", bar(percent(s.MemUsed, s.MemTotal), 18), humanBytes(s.MemUsed), humanBytes(s.MemTotal))
	if s.SwapTotal > 0 {
		fmt.Printf("Swap:   %s  %s / %s\n", bar(percent(s.SwapUsed, s.SwapTotal), 18), humanBytes(s.SwapUsed), humanBytes(s.SwapTotal))
	}
	fmt.Printf("GPU:    Temp %-7s VCN: %s\n", s.GPUTemp, status(s.VCN))
	fmt.Printf("Net:    %s (%s)  ↓ %s/s  ↑ %s/s\n", s.Interface, emptyAs(s.IP, "no IP"), humanRate(s.RXPerSec), humanRate(s.TXPerSec))

	fmt.Println(dim(strings.Repeat("─", width)))
	fmt.Println(bold("Disk Usage"))
	for _, d := range s.Disks {
		fmt.Printf("%-9s %s  %6s / %-6s  %s\n", d.Path, bar(parsePercent(d.Percent), 16), d.Used, d.Size, d.Percent)
	}

	fmt.Println(dim(strings.Repeat("─", width)))
	fmt.Println(bold("Docker"))
	if len(s.Docker) == 0 {
		fmt.Println(dim("no containers or docker unavailable"))
	}
	max := len(s.Docker)
	if max > 8 {
		max = 8
	}
	for i := 0; i < max; i++ {
		fmt.Printf("%-28s %s\n", truncate(s.Docker[i].Name, 28), dockerStatus(s.Docker[i].Status))
	}
	if len(s.Docker) > max {
		fmt.Printf("%s\n", dim(fmt.Sprintf("+%d more", len(s.Docker)-max)))
	}

	fmt.Println(dim(strings.Repeat("─", width)))
	if d, ok := findDisk(s.Disks, storage); ok {
		fmt.Printf("Storage %s: %s %s\n", storage, bar(parsePercent(d.Percent), 24), d.Percent)
	}
	fmt.Print(dim("Ctrl+C to exit"))
}

func readCPU() (CPUStat, uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return CPUStat{}, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return CPUStat{}, 0
	}
	p := strings.Fields(sc.Text())
	if len(p) < 6 {
		return CPUStat{}, 0
	}
	var vals []uint64
	for _, x := range p[1:] {
		v, _ := strconv.ParseUint(x, 10, 64)
		vals = append(vals, v)
	}
	var total uint64
	for _, v := range vals {
		total += v
	}
	idle := vals[3]
	iow := vals[4]
	return CPUStat{Total: total, Idle: idle + iow}, iow
}

func readUptime() string {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "N/A"
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return "N/A"
	}
	sec, _ := strconv.ParseFloat(f[0], 64)
	d := time.Duration(sec) * time.Second
	days := int(d / (24 * time.Hour))
	d %= 24 * time.Hour
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, h, m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

func readMem() (uint64, uint64, uint64, uint64) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, 0
	}
	vals := map[string]uint64{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		p := strings.Fields(sc.Text())
		if len(p) >= 2 {
			v, _ := strconv.ParseUint(p[1], 10, 64)
			vals[strings.TrimSuffix(p[0], ":")] = v * 1024
		}
	}
	total := vals["MemTotal"]
	used := total - vals["MemAvailable"]
	st := vals["SwapTotal"]
	su := st - vals["SwapFree"]
	return used, total, su, st
}

func readLoad() (string, string, string) {
	b, e := os.ReadFile("/proc/loadavg")
	if e != nil {
		return "?", "?", "?"
	}
	p := strings.Fields(string(b))
	if len(p) < 3 {
		return "?", "?", "?"
	}
	return p[0], p[1], p[2]
}

func readCPUTemp() string {
	candidates, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	hw, _ := filepath.Glob("/sys/class/hwmon/hwmon*/temp1_input")
	candidates = append(candidates, hw...)
	for _, p := range candidates {
		if b, e := os.ReadFile(p); e == nil {
			v, _ := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
			if v > 1000 {
				v /= 1000
			}
			if v > 5 && v < 130 {
				return fmt.Sprintf("%.0f°C", v)
			}
		}
	}
	return "N/A"
}

func readGPU(ctx context.Context) (string, string) {
	helper := envOr("GPU_INFO_HELPER", "/usr/local/sbin/nas-gpu-info")
	if _, e := os.Stat(helper); e != nil {
		return "N/A", "N/A"
	}
	cctx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	defer cancel()
	out, e := exec.CommandContext(cctx, "sudo", "-n", helper).Output()
	if e != nil {
		return "N/A", "N/A"
	}
	temp, vcn := "N/A", "N/A"
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "GPU Temperature:") {
			p := strings.Fields(line)
			if len(p) >= 2 {
				temp = p[len(p)-2] + "°C"
			}
		}
		if strings.HasPrefix(line, "VCN:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "VCN:"))
			if strings.Contains(v, "Powered down") {
				vcn = "IDLE"
			} else if v != "" {
				vcn = "ACTIVE"
			}
		}
	}
	return temp, vcn
}

func detectInterface(preferred string) string {
	if _, e := os.Stat("/sys/class/net/" + preferred); e == nil {
		return preferred
	}
	out, e := exec.Command("sh", "-c", "ip route show default | awk '/default/{print $5; exit}'").Output()
	if e == nil && strings.TrimSpace(string(out)) != "" {
		return strings.TrimSpace(string(out))
	}
	return preferred
}
func readIP(ctx context.Context, iface string) string {
	cctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	out, e := exec.CommandContext(cctx, "ip", "-4", "-o", "addr", "show", "dev", iface).Output()
	if e != nil {
		return ""
	}
	p := strings.Fields(string(out))
	for i, x := range p {
		if x == "inet" && i+1 < len(p) {
			return strings.Split(p[i+1], "/")[0]
		}
	}
	return ""
}
func readNet(iface string) NetStat {
	rx := readUint("/sys/class/net/" + iface + "/statistics/rx_bytes")
	tx := readUint("/sys/class/net/" + iface + "/statistics/tx_bytes")
	return NetStat{RX: rx, TX: tx}
}
func readUint(path string) uint64 {
	b, e := os.ReadFile(path)
	if e != nil {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	return v
}

func readDisks(ctx context.Context, paths []string) []DiskUsage {
	var out []DiskUsage
	for _, p := range paths {
		if _, e := os.Stat(p); e != nil {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, time.Second)
		b, e := exec.CommandContext(cctx, "df", "-hP", p).Output()
		cancel()
		if e != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(lines) < 2 {
			continue
		}
		f := strings.Fields(lines[len(lines)-1])
		if len(f) >= 6 {
			out = append(out, DiskUsage{Path: p, FS: f[0], Size: f[1], Used: f[2], Avail: f[3], Percent: f[4]})
		}
	}
	return out
}
func readDocker(ctx context.Context) []DockerService {
	cctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	b, e := exec.CommandContext(cctx, "docker", "ps", "--format", "{{.Names}}\t{{.Status}}").Output()
	if e != nil {
		return nil
	}
	var out []DockerService
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		p := strings.SplitN(sc.Text(), "\t", 2)
		if len(p) == 2 {
			out = append(out, DockerService{Name: p[0], Status: p[1]})
		}
	}
	return out
}

func terminalWidth() int {
	if c := os.Getenv("COLUMNS"); c != "" {
		if n, e := strconv.Atoi(c); e == nil && n > 0 {
			return n
		}
	}
	out, e := exec.Command("sh", "-c", "stty size </dev/tty 2>/dev/null").Output()
	if e == nil {
		p := strings.Fields(string(out))
		if len(p) == 2 {
			n, _ := strconv.Atoi(p[1])
			if n > 0 {
				return n
			}
		}
	}
	return 80
}
func clearScreen()           { fmt.Print("\x1b[H\x1b[2J") }
func hideCursor()            { fmt.Print("\x1b[?25l") }
func showCursor()            { fmt.Print("\x1b[?25h") }
func bold(s string) string   { return "\x1b[1m" + s + "\x1b[0m" }
func cyan(s string) string   { return "\x1b[36m" + s + "\x1b[0m" }
func green(s string) string  { return "\x1b[32m" + s + "\x1b[0m" }
func yellow(s string) string { return "\x1b[33m" + s + "\x1b[0m" }
func red(s string) string    { return "\x1b[31m" + s + "\x1b[0m" }
func dim(s string) string    { return "\x1b[90m" + s + "\x1b[0m" }
func status(s string) string {
	switch s {
	case "ACTIVE":
		return green(s)
	case "IDLE":
		return yellow(s)
	default:
		return dim(s)
	}
}
func dockerStatus(s string) string {
	l := strings.ToLower(s)
	if strings.Contains(l, "healthy") {
		return green(s)
	}
	if strings.HasPrefix(l, "up") {
		return green(s)
	}
	if strings.Contains(l, "unhealthy") || strings.Contains(l, "exited") {
		return red(s)
	}
	return yellow(s)
}
func percent(a, b uint64) int {
	if b == 0 {
		return 0
	}
	return int(a * 100 / b)
}
func bar(p, w int) string {
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	n := p * w / 100
	return "[" + strings.Repeat("█", n) + strings.Repeat("░", w-n) + "]"
}
func parsePercent(s string) int  { n, _ := strconv.Atoi(strings.TrimSuffix(s, "%")); return n }
func humanBytes(v uint64) string { return humanRate(float64(v)) }
func humanRate(v float64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f%s", v, units[i])
	}
	return fmt.Sprintf("%.1f%s", v, units[i])
}
func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func emptyAs(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 2 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
func fit(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s + strings.Repeat(" ", n-len(r))
}
func findDisk(ds []DiskUsage, path string) (DiskUsage, bool) {
	for _, d := range ds {
		if d.Path == path {
			return d, true
		}
	}
	return DiskUsage{}, false
}
