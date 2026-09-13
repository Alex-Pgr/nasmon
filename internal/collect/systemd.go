package collect

import (
	"strings"

	"nasmon/internal/model"
)

func CollectSystemd(store *model.Store) {
	out, err := commandOutput("systemctl", "--failed", "--no-legend", "--plain")
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
