package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

func dockerMemoryLabel(b uint64) string {
	if b == 0 {
		return "-"
	}
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0fM", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0fK", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func dockerStatus(c model.Container) string {
	txt := c.Status
	if c.Health != "" {
		txt = strings.ReplaceAll(txt, " ("+c.Health+")", "")
		txt = strings.ReplaceAll(txt, " (health: "+c.Health+")", "")
	}
	txt = strings.NewReplacer(
		" seconds", "s",
		" second", "s",
		" minutes", "m",
		" minute", "m",
		" hours", "h",
		" hour", "h",
	).Replace(txt)
	return strings.TrimSpace(txt)
}

func selectDockerContainers(containers []model.Container, limit int) []model.Container {
	if limit <= 0 {
		return nil
	}
	if len(containers) <= limit {
		return append([]model.Container(nil), containers...)
	}

	restarted := make([]model.Container, 0)
	others := make([]model.Container, 0)
	for _, c := range containers {
		if c.Restarts > 0 {
			restarted = append(restarted, c)
		} else {
			others = append(others, c)
		}
	}

	sort.SliceStable(restarted, func(i, j int) bool {
		if restarted[i].Restarts != restarted[j].Restarts {
			return restarted[i].Restarts > restarted[j].Restarts
		}
		if restarted[i].MemoryBytes != restarted[j].MemoryBytes {
			return restarted[i].MemoryBytes > restarted[j].MemoryBytes
		}
		return strings.ToLower(restarted[i].Name) < strings.ToLower(restarted[j].Name)
	})
	sort.SliceStable(others, func(i, j int) bool {
		if others[i].MemoryBytes != others[j].MemoryBytes {
			return others[i].MemoryBytes > others[j].MemoryBytes
		}
		return strings.ToLower(others[i].Name) < strings.ToLower(others[j].Name)
	})

	out := make([]model.Container, 0, limit)
	for _, list := range [][]model.Container{restarted, others} {
		for _, c := range list {
			if len(out) == limit {
				return out
			}
			out = append(out, c)
		}
	}
	return out
}
