package collect

import (
	"bufio"
	"os"
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

const diskPowerSudoBackoff = 10 * time.Minute

var (
	diskActivityMu sync.Mutex
	diskActivity   = map[string]diskActivityState{}

	diskPowerMu         sync.Mutex
	diskPowerRetryAfter = map[string]time.Time{}
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
	out, err := commandCombinedOutput("smartctl", args...)
	if err != nil && smartctlPermissionError(string(out)) {
		out, err = commandCombinedOutput("sudo", append([]string{"-n", "smartctl"}, args...)...)
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

func hdparmPermissionError(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "permission denied") || strings.Contains(low, "operation not permitted")
}

func hdparmSudoUnavailable(text string) bool {
	low := strings.ToLower(text)
	return strings.Contains(low, "password is required") ||
		strings.Contains(low, "a terminal is required") ||
		strings.Contains(low, "no tty present") ||
		strings.Contains(low, "not allowed to execute")
}

func diskPowerRetryAllowed(dev string) bool {
	diskPowerMu.Lock()
	defer diskPowerMu.Unlock()
	retryAfter, ok := diskPowerRetryAfter[dev]
	return !ok || time.Now().After(retryAfter)
}

func setDiskPowerRetryBackoff(dev string) {
	diskPowerMu.Lock()
	diskPowerRetryAfter[dev] = time.Now().Add(diskPowerSudoBackoff)
	diskPowerMu.Unlock()
}

func clearDiskPowerRetryBackoff(dev string) {
	diskPowerMu.Lock()
	delete(diskPowerRetryAfter, dev)
	diskPowerMu.Unlock()
}

func diskPowerState(dev string) (bool, bool) {
	args := []string{"-C", "/dev/" + dev}
	out, err := commandCombinedOutput("hdparm", args...)
	if err != nil && hdparmPermissionError(string(out)) {
		if !diskPowerRetryAllowed(dev) {
			return false, false
		}
		out, err = commandCombinedOutput("sudo", "-n", "hdparm", "-C", "/dev/"+dev)
		if err != nil && hdparmSudoUnavailable(string(out)) {
			setDiskPowerRetryBackoff(dev)
			return false, false
		}
		if err == nil {
			clearDiskPowerRetryBackoff(dev)
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

type diskHealthMerge func(*model.DiskHealth, model.DiskHealth)

func mergeDiskHealth(store *model.Store, devs []string, updates map[string]model.DiskHealth, merge diskHealthMerge) {
	store.Update(func(s *model.Snapshot) {
		current := make(map[string]model.DiskHealth, len(s.DiskHealth))
		for _, h := range s.DiskHealth {
			current[h.Device] = h
		}
		out := make([]model.DiskHealth, 0, len(devs))
		for _, dev := range devs {
			h := current[dev]
			h.Device = dev
			if update, ok := updates[dev]; ok {
				merge(&h, update)
			}
			out = append(out, h)
		}
		s.DiskHealth = out
	})
}

func mergePowerHealth(dst *model.DiskHealth, src model.DiskHealth) {
	dst.Sleeping = src.Sleeping
}

func mergeTemperatureHealth(dst *model.DiskHealth, src model.DiskHealth) {
	if src.Sleeping {
		if dst.Temperature == "" {
			dst.Temperature = "N/A"
		}
		return
	}
	dst.Temperature = src.Temperature
}

func mergeSMARTHealth(dst *model.DiskHealth, src model.DiskHealth) {
	dst.Health = src.Health
	dst.NVMe = src.NVMe
	dst.NVMeMetrics = src.NVMeMetrics
	dst.PercentageUsed = src.PercentageUsed
	dst.AvailableSpare = src.AvailableSpare
	dst.SpareThreshold = src.SpareThreshold
	dst.CriticalWarning = src.CriticalWarning
	dst.MediaErrors = src.MediaErrors
	dst.ErrorLogEntries = src.ErrorLogEntries
	dst.Reallocated = src.Reallocated
	dst.Pending = src.Pending
	dst.Uncorrect = src.Uncorrect
}

// CollectDiskPowerState uses ATA CHECK POWER MODE through hdparm -C. This does
// not spin up a standby disk and is intentionally independent from SMART data,
// so the last known health/temperature remain visible while the drive sleeps.
func CollectDiskPowerState(devs []string, store *model.Store) {
	updates := map[string]model.DiskHealth{}
	for _, d := range devs {
		if !isRotational(d) {
			updates[d] = model.DiskHealth{Sleeping: false}
			continue
		}
		asleep, ok := diskPowerState(d)
		if ok {
			updates[d] = model.DiskHealth{Sleeping: asleep}
		}
	}
	mergeDiskHealth(store, devs, updates, mergePowerHealth)
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

func smartMetric(text, label string) (string, bool) {
	prefix := label + ":"
	for _, ln := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)), true
		}
	}
	return "", false
}

func parseMetricUint(raw string) (uint64, bool) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return 0, false
	}
	token := strings.TrimSuffix(fields[0], "%")
	token = strings.ReplaceAll(token, ",", "")
	base := 10
	if strings.HasPrefix(strings.ToLower(token), "0x") {
		base = 0
	}
	v, err := strconv.ParseUint(token, base, 64)
	return v, err == nil
}

func parseNVMeSMART(text string, h *model.DiskHealth) {
	h.NVMe = true
	parsed := false
	if raw, ok := smartMetric(text, "Critical Warning"); ok {
		if v, valid := parseMetricUint(raw); valid {
			h.CriticalWarning = v
			parsed = true
		}
	}
	if raw, ok := smartMetric(text, "Available Spare"); ok {
		if v, valid := parseMetricUint(raw); valid {
			h.AvailableSpare = int(v)
			parsed = true
		}
	}
	if raw, ok := smartMetric(text, "Available Spare Threshold"); ok {
		if v, valid := parseMetricUint(raw); valid {
			h.SpareThreshold = int(v)
			parsed = true
		}
	}
	if raw, ok := smartMetric(text, "Percentage Used"); ok {
		if v, valid := parseMetricUint(raw); valid {
			h.PercentageUsed = int(v)
			parsed = true
		}
	}
	if raw, ok := smartMetric(text, "Media and Data Integrity Errors"); ok {
		if v, valid := parseMetricUint(raw); valid {
			h.MediaErrors = v
			parsed = true
		}
	}
	if raw, ok := smartMetric(text, "Error Information Log Entries"); ok {
		if v, valid := parseMetricUint(raw); valid {
			h.ErrorLogEntries = v
			parsed = true
		}
	}
	if parsed {
		h.NVMeMetrics = true
	}
}

func nvmeHealthWarning(h model.DiskHealth) bool {
	if h.CriticalWarning != 0 || h.MediaErrors > 0 {
		return true
	}
	if h.NVMeMetrics && h.PercentageUsed >= 100 {
		return true
	}
	return h.SpareThreshold > 0 && h.AvailableSpare < h.SpareThreshold
}

func CollectDiskTemps(devs []string, quietWindow time.Duration, store *model.Store) {
	updates := map[string]model.DiskHealth{}
	for _, d := range devs {
		if !shouldPollDisk(d, quietWindow) {
			continue
		}
		txt, _ := smartctl(d, "-n", "standby,0", "-A")
		if sleeping(txt) {
			updates[d] = model.DiskHealth{Sleeping: true}
		} else {
			updates[d] = model.DiskHealth{Temperature: parseTemp(txt)}
		}
	}
	mergeDiskHealth(store, devs, updates, mergeTemperatureHealth)
}

func CollectSMART(devs []string, quietWindow time.Duration, store *model.Store) {
	snap := store.Snapshot()
	previous := map[string]model.DiskHealth{}
	for _, h := range snap.DiskHealth {
		previous[h.Device] = h
	}
	updates := map[string]model.DiskHealth{}
	for _, d := range devs {
		h := previous[d]
		h.Device = d
		h.NVMe = strings.HasPrefix(d, "nvme")
		if !shouldPollDisk(d, quietWindow) {
			continue
		}
		txt, _ := smartctl(d, "-n", "standby,0", "-H", "-A")
		if sleeping(txt) {
			if h.Health == "" {
				h.Health = "PENDING"
			}
			updates[d] = h
			continue
		}
		switch {
		case strings.Contains(txt, "PASSED") || strings.Contains(txt, "SMART Health Status: OK"):
			h.Health = "OK"
		case strings.Contains(txt, "SMART overall-health") || strings.Contains(txt, "SMART Health Status:"):
			h.Health = "WARN"
		default:
			h.Health = "N/A"
		}

		if h.NVMe {
			parseNVMeSMART(txt, &h)
			if nvmeHealthWarning(h) {
				h.Health = "WARN"
			}
		} else {
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
		}
		updates[d] = h
	}
	mergeDiskHealth(store, devs, updates, mergeSMARTHealth)
}
