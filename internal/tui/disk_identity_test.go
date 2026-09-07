package tui

import (
	"testing"

	"nasmon/internal/model"
)

func TestNormalizeDiskBrand(t *testing.T) {
	cases := []struct {
		vendor string
		model  string
		want   string
	}{
		{"", "MZALQ256HBJD-00BL1", "Samsung"},
		{"", "ADATA SX6000PNP", "ADATA"},
		{"ATA", "ST1000LM035-1RK172", "Seagate"},
		{"ATA", "WDC WD40EFRX", "WD"},
	}
	for _, tc := range cases {
		if got := normalizeDiskBrand(tc.vendor, tc.model); got != tc.want {
			t.Fatalf("normalizeDiskBrand(%q, %q) = %q, want %q", tc.vendor, tc.model, got, tc.want)
		}
	}
}

func TestShortDiskID(t *testing.T) {
	cases := map[string]string{
		"nvme0n1":  "0n1",
		"nvme1n1":  "1n1",
		"nvme10n1": "10n1",
		"sda":      "sda",
	}
	for in, want := range cases {
		if got := shortDiskID(in); got != want {
			t.Fatalf("shortDiskID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecorateDiskHealthAlignsBrands(t *testing.T) {
	s := model.Snapshot{DiskHealth: []model.DiskHealth{
		{Device: "nvme0n1"},
		{Device: "nvme1n1"},
		{Device: "sda"},
	}}
	brands := map[string]string{
		"nvme0n1": "Samsung",
		"nvme1n1": "ADATA",
		"sda":     "Seagate",
	}
	decorateDiskHealthWithBrand(&s, func(device string) string { return brands[device] })
	want := []string{"Samsung (0n1)", "ADATA   (1n1)", "Seagate (sda)"}
	for i, h := range s.DiskHealth {
		if h.Device != want[i] {
			t.Fatalf("DiskHealth[%d].Device = %q, want %q", i, h.Device, want[i])
		}
	}
}
