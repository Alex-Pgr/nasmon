package collect

import (
	"nasmon/internal/model"
	"os/exec"
	"strings"
)

func CollectSystemd(store *model.Store) {
	out, err := exec.Command("systemctl", "--failed", "--no-legend", "--plain").Output()
	if err != nil {
		return
	}
	n := 0
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	store.Update(func(s *model.Snapshot) { s.FailedUnits = n })
}
