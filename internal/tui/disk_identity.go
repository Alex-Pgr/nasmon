package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

func shortDiskID(device string) string {
	if strings.HasPrefix(device, "nvme") {
		id := strings.TrimPrefix(device, "nvme")
		if n := utf8.RuneCountInString(id); n >= 3 && n <= 4 {
			return id
		}
	}
	r := []rune(device)
	if len(r) <= 4 {
		return device
	}
	return string(r[len(r)-3:])
}

func diskHealthLabels(health []model.DiskHealth) []string {
	brands := make([]string, len(health))
	brandW := 0
	for i, h := range health {
		brand := strings.TrimSpace(h.Brand)
		if brand == "" {
			brand = "Disk"
		}
		brands[i] = brand
		if n := cellWidth(brand); n > brandW {
			brandW = n
		}
	}

	labels := make([]string, len(health))
	for i, h := range health {
		pad := brandW - cellWidth(brands[i])
		labels[i] = brands[i] + strings.Repeat(" ", pad) + " (" + shortDiskID(h.Device) + ")"
	}
	return labels
}
