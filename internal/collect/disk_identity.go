package collect

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"nasmon/internal/model"
)

type diskIdentityValue struct {
	vendor string
	model  string
	brand  string
}

var diskIdentityCache sync.Map

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

func diskIdentity(device string) diskIdentityValue {
	if cached, ok := diskIdentityCache.Load(device); ok {
		return cached.(diskIdentityValue)
	}
	base := filepath.Join("/sys/class/block", device, "device")
	vendor := readSysfsText(filepath.Join(base, "vendor"))
	modelName := readSysfsText(filepath.Join(base, "model"))
	identity := diskIdentityValue{
		vendor: vendor,
		model:  modelName,
		brand:  normalizeDiskBrand(vendor, modelName),
	}
	diskIdentityCache.Store(device, identity)
	return identity
}

func populateDiskIdentity(h *model.DiskHealth) {
	if h.Device == "" || (h.Vendor != "" && h.Model != "" && h.Brand != "") {
		return
	}
	identity := diskIdentity(h.Device)
	h.Vendor = identity.vendor
	h.Model = identity.model
	h.Brand = identity.brand
}
