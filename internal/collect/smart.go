package collect

import (
	"bufio"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"nasmon/internal/model"
)

type diskActivityState struct {
	readSectors  uint64
	writeSectors uint64
	lastActivity time.Time
	initialized  bool
}

var (
	diskActivityMu sync.Mutex
	diskActivity   = map[string]diskActivityState{}
)

func RefreshDiskActivity(devs []string) {
	if len(devs) == 0 {
		return
	}
	wanted := map[string]bool{}
	for _, d := range devs {
		wanted[d] = true
	}

	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return
	}
	defer f.Close()

	type counters struct{ read, write uint64 }
	current := map[string]counters{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 || !wanted[fields[2]] {
			continue
		}
		r, errR := strconv.ParseUint(fields[5], 10, 64)
		w, errW := strconv.ParseUint(fields[9], 10, 64)
		if errR != nil || errW != nil {
			continue
		}
		current[fields[2]] = counters{read: r, write: w}
	}

	now := time.Now()
	diskActivityMu.Lock()
	defer diskActivityMu.Unlock()
	for _, d := range devs {
		c, ok := current[d]
		if !ok {
			continue
		}
		st := diskActivity[d]
		if !st.initialized {
			st.initialized = true
			st.lastActivity = now
		} else if c.read != st.readSectors || c.write != st.writeSectors {
			st.lastActivity = now
		}
		st.readSectors = c.read
		st.writeSectors = c.write
		diskActivity[d] = st
	}
	for d := range diskActivity {
		if !wanted[d] {
			delete(diskActivity, d)
		}
	}
}

func isRotational(dev string) bool {
	b, err := os.ReadFile("/sys/class/block/" + dev + "/queue/rotational")
	if err != nil {
		return true
	}
	return strings.TrimSpace(string(b)) != "0"
}

func shouldPollDisk(dev string, quietWindow time.Duration) bool {
	if quietWindow <= 0 || !isRotational(dev) {
		return true
	}
	diskActivityMu.Lock()
	st, ok := diskActivity[dev]
	diskActivityMu.Unlock()
	if !ok || !st.initialized || st.lastActivity.IsZero() {
		return true
	}
	return time.Since(st.lastActivity) < quietWindow
}

func smartctlPermissionError(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "permission denied") || strings.Contains(low, "must be root") || strings.Contains(low, "operation not permitted")
}

func runSmartctl(args []string) (string, error) {
	out, err := exec.Command("smartctl", args...).CombinedOutput()
	if err != nil && smartctlPermissionError(string(out)) {
		out, err = exec.Command("sudo", append([]string{"-n", "smartctl"}, args...)...).CombinedOutput()
	}
	return string(out), err
}

func smartctl(dev string, args ...string) (string, error) {
	full := append(append([]string(nil), args...), "/dev/"+dev)
	txt, err := runSmartctl(full)
	low := strings.ToLower(txt)
	if strings.Contains(low, "unknown usb bridge") || strings.Contains(low, "please specify device type with the -d option") {
		sat := append([]string{"-d", "sat"}, args...)
		sat = append(sat, "/dev/"+dev)
		return runSmartctl(sat)
	}
	return txt, err
}

func diskPowerState(dev string) (bool, bool) {
	args := []string{"-C", "/dev/" + dev}
	out, err := exec.Command("hdparm", args...).CombinedOutput()
	if err != nil {
		low := strings.ToLower(string(out))
		if strings.Contains(low, "permission denied") || strings.Contains(low, "operation not permitted") {
			out, err = exec.Command("sudo", "-n", "hdparm", "-C", "/dev/"+dev).CombinedOutput()
		}
	}
	if err != nil {
		return false, false
	}
	low := strings.ToLower(string(out))
	if strings.Contains(low, "standby") {
		return true, true
	}
	if strings.Contains(low, "active/idle") || strings.Contains(low, "active") || strings.Contains(low, "idle") {
		return false, true
	}
	return false, false
}

// CollectDiskPowerState uses ATA CHECK POWER MODE through hdparm -C. This does
// not spin up a standby disk and is intentionally independent from SMART data,
// so the last known health/temperature remain visible while the drive sleeps.
func CollectDiskPowerState(devs []string, store *model.Store) {
	snap := store.Snapshot()
	m := map[string]model.DiskHealth{}
	for _, h := range snap.DiskHealth {
		m[h.Device] = h
	}
	for _, d := range devs {
		h := m[d]
		h.Device = d
		if !isRotational(d) {
			h.Sleeping = false
			m[d] = h
			continue
		}
		asleep, ok := diskPowerState(d)
		if ok {
			h.Sleeping = asleep
		}
		m[d] = h
	}
	out := make([]model.DiskHealth, 0, len(devs))
	for _, d := range devs {
		out = append(out, m[d])
	}
	store.Update(func(s *model.Snapshot) { s.DiskHealth = out })
}

func sleeping(text string) bool {
	l := strings.ToLower(text)
	return strings.Contains(l, "standby") || strings.Contains(l, "sleep mode") || strings.Contains(l, "low-power mode")
}
func parseTemp(text string) string {
	for _, ln := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "Temperature:") {
			p := strings.Fields(trimmed)
			if len(p) >= 2 {
				if _, e := strconv.Atoi(p[1]); e == nil {
					return p[1] + "°C"
				}
			}
		}
		if strings.Contains(ln, "Temperature_Celsius") || strings.Contains(ln, "Temperature_Case") || strings.Contains(ln, "Airflow_Temperature_Cel") || strings.Contains(ln, "Temperature_Internal") {
			p := strings.Fields(ln)
			if len(p) >= 10 {
				if _, e := strconv.Atoi(p[9]); e == nil {
					return p[9] + "°C"
				}
			}
		}
	}
	return "N/A"
}
func CollectDiskTemps(devs []string, quietWindow time.Duration, store *model.Store) {
	snap := store.Snapshot()
	m := map[string]model.DiskHealth{}
	for _, h := range snap.DiskHealth {
		m[h.Device] = h
	}
	for _, d := range devs {
		h := m[d]
		h.Device = d
		if !shouldPollDisk(d, quietWindow) {
			m[d] = h
			continue
		}
		txt, _ := smartctl(d, "-n", "standby,0", "-A")
		if sleeping(txt) {
			h.Sleeping = true
			if h.Temperature == "" {
				h.Temperature = "N/A"
			}
		} else {
			h.Sleeping = false
			h.Temperature = parseTemp(txt)
		}
		m[d] = h
	}
	out := make([]model.DiskHealth, 0, len(devs))
	for _, d := range devs {
		out = append(out, m[d])
	}
	store.Update(func(s *model.Snapshot) { s.DiskHealth = out })
}
func CollectSMART(devs []string, quietWindow time.Duration, store *model.Store) {
	snap := store.Snapshot()
	m := map[string]model.DiskHealth{}
	for _, h := range snap.DiskHealth {
		m[h.Device] = h
	}
	for _, d := range devs {
		h := m[d]
		h.Device = d
		if !shouldPollDisk(d, quietWindow) {
			m[d] = h
			continue
		}
		txt, _ := smartctl(d, "-n", "standby,0", "-H", "-A")
		if sleeping(txt) {
			h.Sleeping = true
			if h.Health == "" {
				h.Health = "PENDING"
			}
			m[d] = h
			continue
		}
		h.Sleeping = false
		switch {
		case strings.Contains(txt, "PASSED") || strings.Contains(txt, "SMART Health Status: OK"):
			h.Health = "OK"
		case strings.Contains(txt, "SMART overall-health") || strings.Contains(txt, "SMART Health Status:"):
			h.Health = "WARN"
		default:
			h.Health = "N/A"
		}
		for _, ln := range strings.Split(txt, "\n") {
			f := strings.Fields(ln)
			if len(f) < 10 {
				continue
			}
			if f[0] != "5" && f[0] != "197" && f[0] != "198" {
				continue
			}
			v, err := strconv.ParseInt(f[9], 10, 64)
			if err != nil {
				continue
			}
			switch f[0] {
			case "5":
				h.Reallocated = v
			case "197":
				h.Pending = v
			case "198":
				h.Uncorrect = v
			}
		}
		m[d] = h
	}
	out := make([]model.DiskHealth, 0, len(devs))
	for _, d := range devs {
		out = append(out, m[d])
	}
	store.Update(func(s *model.Snapshot) { s.DiskHealth = out })
}
