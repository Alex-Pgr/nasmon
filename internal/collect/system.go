package collect

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"nasmon/internal/model"
)

type CPUCollector struct {
	mu          sync.Mutex
	prevTotal   uint64
	prevIdle    uint64
	prevIOWait  uint64
	initialized bool
}

func parseCPUStat() (total, idleAll, iowait uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, 0, fmt.Errorf("/proc/stat empty")
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 6 || fields[0] != "cpu" {
		return 0, 0, 0, fmt.Errorf("bad cpu line")
	}
	vals := make([]uint64, len(fields)-1)
	for i := 1; i < len(fields); i++ {
		vals[i-1], _ = strconv.ParseUint(fields[i], 10, 64)
	}
	for i, v := range vals {
		if i == 8 || i == 9 {
			continue
		} // guest, guest_nice already included in user/nice
		total += v
	}
	idle := vals[3]
	if len(vals) > 4 {
		iowait = vals[4]
	}
	idleAll = idle + iowait
	return
}

func (c *CPUCollector) Collect(store *model.Store) {
	total, idle, iowait, err := parseCPUStat()
	if err != nil {
		return
	}

	c.mu.Lock()
	usage, ioPct := 0, 0
	if c.initialized && total > c.prevTotal {
		td := total - c.prevTotal
		id := idle - c.prevIdle
		iw := iowait - c.prevIOWait
		usage = 100 - int(id*100/td)
		ioPct = int(iw * 100 / td)
		if usage < 0 {
			usage = 0
		}
		if usage > 100 {
			usage = 100
		}
		if ioPct < 0 {
			ioPct = 0
		}
		if ioPct > 100 {
			ioPct = 100
		}
	} else {
		c.initialized = true
	}
	c.prevTotal, c.prevIdle, c.prevIOWait = total, idle, iowait
	c.mu.Unlock()

	store.Update(func(s *model.Snapshot) {
		s.CPUUsage = usage
		s.IOWait = ioPct
	})
}

func collectZRAMSwap() (total, used uint64) {
	f, err := os.Open("/proc/swaps")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 || !strings.HasPrefix(filepath.Base(fields[0]), "zram") {
			continue
		}
		sizeKiB, errSize := strconv.ParseUint(fields[2], 10, 64)
		usedKiB, errUsed := strconv.ParseUint(fields[3], 10, 64)
		if errSize != nil || errUsed != nil {
			continue
		}
		total += sizeKiB * 1024
		used += usedKiB * 1024
	}
	return total, used
}

func CollectMemory(store *model.Store) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()

	vals := map[string]uint64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.Fields(sc.Text())
		if len(p) < 2 {
			continue
		}
		switch p[0] {
		case "MemTotal:", "MemAvailable:", "SwapTotal:", "SwapFree:":
			n, _ := strconv.ParseUint(p[1], 10, 64)
			vals[p[0]] = n * 1024
		}
	}
	mt, ma := vals["MemTotal:"], vals["MemAvailable:"]
	st, sf := vals["SwapTotal:"], vals["SwapFree:"]
	mu, su := uint64(0), uint64(0)
	if mt >= ma {
		mu = mt - ma
	}
	if st >= sf {
		su = st - sf
	}
	mp, sp := 0, 0
	if mt > 0 {
		mp = int(mu * 100 / mt)
	}
	if st > 0 {
		sp = int(su * 100 / st)
	}
	zt, zu := collectZRAMSwap()
	zp := 0
	if zt > 0 {
		zp = int(zu * 100 / zt)
	}
	store.Update(func(s *model.Snapshot) {
		s.MemTotalBytes, s.MemUsedBytes, s.MemPercent = mt, mu, mp
		s.ZRAMTotalBytes, s.ZRAMUsedBytes, s.ZRAMPercent = zt, zu, zp
		s.SwapTotalBytes, s.SwapUsedBytes, s.SwapPercent = st, su, sp
	})
}

func CollectLoadUptime(store *model.Store) {
	upb, err := os.ReadFile("/proc/uptime")
	if err == nil {
		f := strings.Fields(string(upb))
		if len(f) > 0 {
			sec, _ := strconv.ParseFloat(f[0], 64)
			store.Update(func(s *model.Snapshot) { s.Uptime = time.Duration(sec * float64(time.Second)) })
		}
	}
	lb, err := os.ReadFile("/proc/loadavg")
	if err == nil {
		f := strings.Fields(string(lb))
		if len(f) >= 3 {
			store.Update(func(s *model.Snapshot) { s.Load1, s.Load5, s.Load15 = f[0], f[1], f[2] })
		}
	}
}

func readMilliCelsius(path string) (*float64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	raw, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return nil, false
	}
	v := raw / 1000
	return &v, true
}

func hwmonCPUTemp(root string) (preferred, fallback *float64) {
	dirs, _ := filepath.Glob(filepath.Join(root, "hwmon*"))
	for _, d := range dirs {
		nameb, _ := os.ReadFile(filepath.Join(d, "name"))
		name := strings.ToLower(strings.TrimSpace(string(nameb)))
		inputs, _ := filepath.Glob(filepath.Join(d, "temp*_input"))
		for _, input := range inputs {
			v, ok := readMilliCelsius(input)
			if !ok {
				continue
			}
			if fallback == nil {
				fallback = v
			}
			if strings.Contains(name, "k10temp") || strings.Contains(name, "coretemp") {
				return v, fallback
			}
		}
	}
	return nil, fallback
}

func thermalZoneCPUTemp(root string) *float64 {
	zones, _ := filepath.Glob(filepath.Join(root, "thermal_zone*"))
	for _, zone := range zones {
		typeBytes, err := os.ReadFile(filepath.Join(zone, "type"))
		if err != nil {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(string(typeBytes)))
		if !strings.Contains(kind, "cpu") && !strings.Contains(kind, "soc") && !strings.Contains(kind, "package") {
			continue
		}
		if v, ok := readMilliCelsius(filepath.Join(zone, "temp")); ok {
			return v
		}
	}
	return nil
}

func cpuTemperature(hwmonRoot, thermalRoot string) *float64 {
	preferred, fallback := hwmonCPUTemp(hwmonRoot)
	if preferred != nil {
		return preferred
	}
	if thermal := thermalZoneCPUTemp(thermalRoot); thermal != nil {
		return thermal
	}
	return fallback
}

func CollectCPUTemp(store *model.Store) {
	// x86 exposes reliable package sensors through coretemp/k10temp. ARM SBCs
	// commonly expose CPU/SoC temperature through the generic thermal-zone API.
	// A generic hwmon value is retained only as the final fallback.
	temp := cpuTemperature("/sys/class/hwmon", "/sys/class/thermal")
	store.Update(func(s *model.Snapshot) { s.CPUTempC = temp })
}

func maxFanRPM(hwmonRoot string) *int {
	inputs, _ := filepath.Glob(filepath.Join(hwmonRoot, "hwmon*", "fan*_input"))
	found := false
	maxRPM := 0
	for _, in := range inputs {
		b, err := os.ReadFile(in)
		if err != nil {
			continue
		}
		rpm, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil || rpm < 0 {
			continue
		}
		if !found || rpm > maxRPM {
			maxRPM = rpm
			found = true
		}
	}
	if !found {
		return nil
	}
	return &maxRPM
}

func CollectFanRPM(store *model.Store) {
	rpm := maxFanRPM("/sys/class/hwmon")
	store.Update(func(s *model.Snapshot) { s.FanRPM = rpm })
}
