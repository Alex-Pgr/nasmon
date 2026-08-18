package collect

import (
	"os/exec"
	"strconv"
	"strings"

	"nasmon/internal/model"
)

func smartctl(dev string, args ...string) (string, error) {
	full := append(args, "/dev/"+dev)
	out, err := exec.Command("smartctl", full...).CombinedOutput()
	if err == nil {
		return string(out), nil
	}
	low := strings.ToLower(string(out))
	if strings.Contains(low, "permission denied") || strings.Contains(low, "must be root") || strings.Contains(low, "operation not permitted") {
		out, err = exec.Command("sudo", append([]string{"-n", "smartctl"}, full...)...).CombinedOutput()
	}
	return string(out), err
}
func sleeping(text string) bool {
	l := strings.ToLower(text)
	return strings.Contains(l, "standby") || strings.Contains(l, "sleep mode") || strings.Contains(l, "low-power mode")
}
func parseTemp(text string) string {
	for _, ln := range strings.Split(text, "\n") {
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
func CollectDiskTemps(devs []string, store *model.Store) {
	snap := store.Snapshot()
	m := map[string]model.DiskHealth{}
	for _, h := range snap.DiskHealth {
		m[h.Device] = h
	}
	for _, d := range devs {
		txt, _ := smartctl(d, "-n", "standby,0", "-A")
		h := m[d]
		h.Device = d
		if sleeping(txt) {
			h.Sleeping = true
			h.Temperature = "SLEEP"
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
func CollectSMART(devs []string, store *model.Store) {
	snap := store.Snapshot()
	m := map[string]model.DiskHealth{}
	for _, h := range snap.DiskHealth {
		m[h.Device] = h
	}
	for _, d := range devs {
		txt, _ := smartctl(d, "-n", "standby,0", "-H", "-A")
		h := m[d]
		h.Device = d
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
