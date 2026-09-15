package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Alex-Pgr/nas_monitoring/internal/app"
	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

func invariantSnapshot(gpu *int, fan *int, zram bool, containers int) model.Snapshot {
	cpuTemp := 52.0
	gpuTemp := 48.0
	s := model.Snapshot{
		Uptime:            9*time.Hour + 17*time.Minute,
		CPUUsage:          67,
		CPUTempC:          &cpuTemp,
		FanRPM:            fan,
		GPUTempC:          &gpuTemp,
		GPUUsage:          gpu,
		GPUVCN:            "IDLE",
		MemUsedBytes:      15360 << 30,
		MemTotalBytes:     16384 << 30,
		MemPercent:        94,
		SwapUsedBytes:     11 << 30,
		SwapTotalBytes:    16 << 30,
		SwapPercent:       69,
		IOWait:            7,
		Load1:             "1.25",
		Load5:             "1.10",
		Load15:            "0.95",
		IP:                "192.168.100.250",
		NetReady:          true,
		RXBps:             123456789,
		TXBps:             9876543,
		DiskIOReady:       true,
		DiskReadBps:       345678901,
		DiskWriteBps:      23456789,
		StoragePath:       "/mnt/fast",
		StorageUsedBytes:  420 << 30,
		StorageTotalBytes: 468 << 30,
		StoragePercent:    90,
		DiskUsage: []model.DiskUsage{
			{Path: "/", UsedBytes: 31 << 30, TotalBytes: 40 << 30, Percent: 77},
			{Path: "/mnt/fast-with-a-very-long-name", UsedBytes: 420 << 30, TotalBytes: 468 << 30, Percent: 90},
		},
		DiskHealth: []model.DiskHealth{{Device: "/dev/nvme0n1", Temperature: "51°C", Health: "OK"}},
	}
	if zram {
		s.ZRAMUsedBytes = 3 << 30
		s.ZRAMTotalBytes = 4 << 30
		s.ZRAMPercent = 75
	}
	for i := 0; i < containers; i++ {
		s.Containers = append(s.Containers, model.Container{
			ID:          fmt.Sprintf("id-%02d", i),
			Name:        fmt.Sprintf("service-with-an-intentionally-long-container-name-%02d", i),
			State:       "running",
			Status:      "Up 123 hours (healthy)",
			Health:      "healthy",
			MemoryBytes: uint64(i+1) * 257 << 20,
			Restarts:    i % 3,
		})
	}
	return s
}

func assertLandscapeGeometry(t *testing.T, out string, width int) {
	t.Helper()
	const gap = 2
	leftWidth := (width - gap) / 2
	rightStart := leftWidth + gap
	seen := 0
	for _, raw := range strings.Split(out, "\n") {
		plain := stripANSI(raw)
		if plain == "" || cellWidth(plain) != width {
			continue
		}
		runes := []rune(plain)
		if len(runes) <= rightStart {
			continue
		}
		leftBorder := runes[0]
		rightBoxBorder := runes[rightStart]
		if (leftBorder == '┌' || leftBorder == '│' || leftBorder == '└') &&
			(rightBoxBorder == '┌' || rightBoxBorder == '│' || rightBoxBorder == '└') {
			if runes[leftWidth-1] != matchingRightBorder(leftBorder) {
				t.Fatalf("left box border mismatch at cell %d: %q", leftWidth-1, plain)
			}
			if runes[width-1] != matchingRightBorder(rightBoxBorder) {
				t.Fatalf("right box border mismatch at cell %d: %q", width-1, plain)
			}
			seen++
		}
	}
	if seen == 0 {
		t.Fatalf("no landscape box rows found")
	}
}

func matchingRightBorder(left rune) rune {
	switch left {
	case '┌':
		return '┐'
	case '└':
		return '┘'
	default:
		return '│'
	}
}

func TestRenderLayoutInvariants(t *testing.T) {
	zeroGPU := 0
	maxGPU := 100
	rpm := 2200
	viewports := []struct {
		cols int
		rows int
	}{{36, 60}, {60, 60}, {80, 31}, {80, 30}, {120, 30}, {160, 50}}
	scenarios := []struct {
		name string
		s    model.Snapshot
	}{
		{"minimal", invariantSnapshot(nil, nil, false, 0)},
		{"loaded", invariantSnapshot(&zeroGPU, &rpm, true, 20)},
		{"gpu-max", invariantSnapshot(&maxGPU, nil, true, 3)},
	}

	for _, vp := range viewports {
		for _, sc := range scenarios {
			name := fmt.Sprintf("%s/%dx%d", sc.name, vp.cols, vp.rows)
			t.Run(name, func(t *testing.T) {
				cfg := app.Config{MainInterval: 2 * time.Second, MinTermWidth: 36, RightMargin: 1}
				r := Renderer{Config: cfg}
				out := r.RenderInteractive(sc.s, vp.rows, vp.cols, DockerSortDefault)
				_, drawWidth, landscape := effectiveLayout(r, vp.rows, vp.cols)

				for n, raw := range strings.Split(out, "\n") {
					if got := cellWidth(raw); got > drawWidth {
						t.Fatalf("line %d width = %d, viewport draw width = %d: %q", n+1, got, drawWidth, stripANSI(raw))
					}
				}

				labels := []string{"CPU:", "GPU:", "RAM:"}
				if landscape {
					labels = []string{"CPU", "GPU", "RAM"}
				}
				if sc.s.ZRAMTotalBytes > 0 {
					if landscape {
						labels = append(labels, "ZRAM")
					} else {
						labels = append(labels, "ZRAM:")
					}
				}
				assertMetricGridAligned(t, out, labels)

				if landscape {
					assertLandscapeGeometry(t, out, drawWidth)
				}
			})
		}
	}
}
