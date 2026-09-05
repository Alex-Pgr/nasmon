package collect

import (
	"testing"

	"nasmon/internal/model"
)

const nvmeSMARTSample = `SMART overall-health self-assessment test result: PASSED
SMART/Health Information (NVMe Log 0x02)
Critical Warning:                   0x00
Temperature:                        40 Celsius
Available Spare:                    96%
Available Spare Threshold:          10%
Percentage Used:                    7%
Media and Data Integrity Errors:    0
Error Information Log Entries:      12
`

func TestParseNVMeSMART(t *testing.T) {
	h := model.DiskHealth{}
	parseNVMeSMART(nvmeSMARTSample, &h)

	if !h.NVMe || !h.NVMeMetrics {
		t.Fatalf("expected NVMe metrics to be detected: %+v", h)
	}
	if h.CriticalWarning != 0 || h.AvailableSpare != 96 || h.SpareThreshold != 10 || h.PercentageUsed != 7 {
		t.Fatalf("unexpected NVMe percentage/warning metrics: %+v", h)
	}
	if h.MediaErrors != 0 || h.ErrorLogEntries != 12 {
		t.Fatalf("unexpected NVMe error counters: %+v", h)
	}
	if nvmeHealthWarning(h) {
		t.Fatalf("healthy sample must not be classified as warning: %+v", h)
	}
}

func TestNVMeHealthWarning(t *testing.T) {
	cases := []model.DiskHealth{
		{NVMe: true, NVMeMetrics: true, CriticalWarning: 1},
		{NVMe: true, NVMeMetrics: true, MediaErrors: 1},
		{NVMe: true, NVMeMetrics: true, PercentageUsed: 100},
		{NVMe: true, NVMeMetrics: true, AvailableSpare: 9, SpareThreshold: 10},
	}
	for i, h := range cases {
		if !nvmeHealthWarning(h) {
			t.Fatalf("case %d should be warning: %+v", i, h)
		}
	}
}
