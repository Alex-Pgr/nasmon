package collect

import (
	"strings"

	"github.com/Alex-Pgr/nasmon/internal/model"
)

func CollectSystemd(store *model.Store) bool {
	out, err := commandOutput("systemctl", "--failed", "--no-legend", "--plain")
	if err != nil {
		return false
	}
	n := 0
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	store.Update(func(s *model.Snapshot) { s.FailedUnits = n })
	return true
}
