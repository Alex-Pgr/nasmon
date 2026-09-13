package collect

import "testing"

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
		{"NVMe", "UnknownDrive", "UnknownDrive"},
	}
	for _, tc := range cases {
		if got := normalizeDiskBrand(tc.vendor, tc.model); got != tc.want {
			t.Fatalf("normalizeDiskBrand(%q, %q) = %q, want %q", tc.vendor, tc.model, got, tc.want)
		}
	}
}
