package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"nasmon/internal/model"
)

var diskBrandCache sync.Map

func readSysfsText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func normalizeDiskBrand(vendor, modelName string) string {
	vendor = strings.TrimSpace(vendor)
	modelName = strings.TrimSpace(modelName)
	text := strings.ToLower(strings.TrimSpace(vendor + " " + modelName))
	upperModel := strings.ToUpper(modelName)

	switch {
	case strings.Contains(text, "samsung") || strings.HasPrefix(upperModel, "MZ"):
		return "Samsung"
	case strings.Contains(text, "adata") || strings.Contains(upperModel, "SX6000"):
		return "ADATA"
	case strings.Contains(text, "seagate") || (len(upperModel) > 2 && strings.HasPrefix(upperModel, "ST") && upperModel[2] >= '0' && upperModel[2] <= '9'):
		return "Seagate"
	case strings.Contains(text, "western digital") || strings.Contains(text, "wdc") || (len(upperModel) > 2 && strings.HasPrefix(upperModel, "WD") && upperModel[2] >= '0' && upperModel[2] <= '9'):
		return "WD"
	case strings.Contains(text, "crucial"):
		return "Crucial"
	case strings.Contains(text, "kingston"):
		return "Kingston"
	case strings.Contains(text, "sandisk"):
		return "SanDisk"
	case strings.Contains(text, "sk hynix") || strings.Contains(text, "hynix"):
		return "SK hynix"
	case strings.Contains(text, "micron"):
		return "Micron"
	case strings.Contains(text, "toshiba"):
		return "Toshiba"
	case strings.Contains(text, "kioxia"):
		return "Kioxia"
	case strings.Contains(text, "intel"):
		return "Intel"
	case strings.Contains(text, "lexar"):
		return "Lexar"
	case strings.Contains(text, "corsair"):
		return "Corsair"
	}

	v := strings.TrimSpace(vendor)
	vl := strings.ToLower(v)
	if v != "" && vl != "ata" && vl != "nvme" && vl != "usb" && vl != "generic" {
		if fields := strings.Fields(v); len(fields) > 0 {
			return fields[0]
		}
	}
	if fields := strings.Fields(modelName); len(fields) > 0 {
		return fields[0]
	}
	return "Disk"
}

func diskBrand(device string) string {
	if v, ok := diskBrandCache.Load(device); ok {
		return v.(string)
	}
	base := filepath.Join("/sys/class/block", device, "device")
	brand := normalizeDiskBrand(readSysfsText(filepath.Join(base, "vendor")), readSysfsText(filepath.Join(base, "model")))
	diskBrandCache.Store(device, brand)
	return brand
}

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

func decorateDiskHealthWithBrand(s *model.Snapshot, brandFor func(string) string) {
	if len(s.DiskHealth) == 0 {
		return
	}
	health := append([]model.DiskHealth(nil), s.DiskHealth...)
	brands := make([]string, len(health))
	brandW := 0
	for i, h := range health {
		brand := brandFor(h.Device)
		if brand == "" {
			brand = "Disk"
		}
		brands[i] = brand
		if n := utf8.RuneCountInString(brand); n > brandW {
			brandW = n
		}
	}
	for i := range health {
		health[i].Device = fmt.Sprintf("%-*s (%s)", brandW, brands[i], shortDiskID(health[i].Device))
	}
	s.DiskHealth = health
}

// DecorateDiskHealth changes only the local snapshot copy used for rendering.
// SMART collection and daemon state keep the real kernel device names.
func DecorateDiskHealth(s *model.Snapshot) {
	decorateDiskHealthWithBrand(s, diskBrand)
}
