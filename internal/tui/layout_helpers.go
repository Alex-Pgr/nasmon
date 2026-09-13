package tui

import "nasmon/internal/model"

func healthDeviceWidth(health []model.DiskHealth) int {
	width := 3
	for _, h := range health {
		if n := cellWidth(h.Device); n > width {
			width = n
		}
	}
	return clamp(width, 3, 16)
}

func regularSystemRows(s model.Snapshot) int {
	rows := 11
	if s.ZRAMTotalBytes > 0 {
		rows++
	}
	return rows
}

// regularFixedRows includes all regular-layout rows except Docker container
// data rows and the optional two-line run footer.
func regularFixedRows(s model.Snapshot, headerRows int) int {
	return headerRows + regularSystemRows(s) + len(s.DiskUsage) + len(s.DiskHealth) + 15
}
