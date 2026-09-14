package collect

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

var gpuTempRE = regexp.MustCompile(`GPU Temperature:\s*([0-9]+(?:\.[0-9]+)?)`)

func readGPUUsage(paths []string) *int {
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			continue
		}
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		return &v
	}
	return nil
}

func CollectGPUUsage(store *model.Store) {
	paths, _ := filepath.Glob("/sys/class/drm/card*/device/gpu_busy_percent")
	usage := readGPUUsage(paths)
	store.Update(func(s *model.Snapshot) { s.GPUUsage = usage })
}

func CollectGPU(helper string, store *model.Store) bool {
	if helper == "" {
		return false
	}
	out, err := commandCombinedOutput("sudo", "-n", helper)
	if err != nil {
		return false
	}
	text := string(out)
	var temp *float64
	if m := gpuTempRE.FindStringSubmatch(text); len(m) == 2 {
		if v, e := strconv.ParseFloat(m[1], 64); e == nil {
			temp = &v
		}
	}
	vcn := "N/A"
	for _, ln := range strings.Split(text, "\n") {
		if strings.HasPrefix(ln, "VCN:") {
			v := strings.TrimSpace(strings.TrimPrefix(ln, "VCN:"))
			switch {
			case strings.Contains(v, "Powered down"):
				vcn = "IDLE"
			case strings.Contains(v, "Powered up"):
				vcn = "ACTIVE"
			case v != "":
				vcn = "ACTIVE"
			}
			break
		}
	}
	store.Update(func(s *model.Snapshot) { s.GPUTempC = temp; s.GPUVCN = vcn })
	return true
}
