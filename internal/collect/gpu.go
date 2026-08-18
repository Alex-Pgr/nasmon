package collect

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"nasmon/internal/model"
)

var gpuTempRE = regexp.MustCompile(`GPU Temperature:\s*([0-9]+(?:\.[0-9]+)?)`)

func CollectGPU(helper string, store *model.Store) {
	if helper == "" {
		return
	}
	out, err := exec.Command("sudo", "-n", helper).CombinedOutput()
	if err != nil {
		return
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
}
