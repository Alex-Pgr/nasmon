package collect

import (
	"testing"
	"time"

	"github.com/Alex-Pgr/nasmon/internal/model"
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

func TestDiskHealthCollectorsMergeOnlyOwnedFields(t *testing.T) {
	store := model.NewStore(time.Now(), "/")
	store.Update(func(s *model.Snapshot) {
		s.DiskHealth = []model.DiskHealth{{
			Device:      "sda",
			Temperature: "41°C",
			Health:      "OK",
			Sleeping:    false,
			Reallocated: 2,
		}}
	})

	mergeDiskHealth(store, []string{"sda"}, map[string]model.DiskHealth{
		"sda": {Sleeping: true, Temperature: "99°C", Health: "WARN", Reallocated: 99},
	}, mergePowerHealth)
	h := store.Snapshot().DiskHealth[0]
	if !h.Sleeping || h.Temperature != "41°C" || h.Health != "OK" || h.Reallocated != 2 {
		t.Fatalf("power merge overwrote unrelated fields: %+v", h)
	}

	mergeDiskHealth(store, []string{"sda"}, map[string]model.DiskHealth{
		"sda": {Temperature: "44°C", Health: "WARN", Sleeping: false, Reallocated: 99},
	}, mergeTemperatureHealth)
	h = store.Snapshot().DiskHealth[0]
	if h.Temperature != "44°C" || !h.Sleeping || h.Health != "OK" || h.Reallocated != 2 {
		t.Fatalf("temperature merge overwrote unrelated fields: %+v", h)
	}

	mergeDiskHealth(store, []string{"sda"}, map[string]model.DiskHealth{
		"sda": {Health: "WARN", Reallocated: 7, Temperature: "12°C", Sleeping: false},
	}, mergeSMARTHealth)
	h = store.Snapshot().DiskHealth[0]
	if h.Health != "WARN" || h.Reallocated != 7 || h.Temperature != "44°C" || !h.Sleeping {
		t.Fatalf("SMART merge overwrote unrelated fields: %+v", h)
	}
}
