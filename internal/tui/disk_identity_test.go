package tui

import (
	"testing"

	"nasmon/internal/model"
)

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

func TestDiskHealthLabelsAlignBrandsWithoutIO(t *testing.T) {
	health := []model.DiskHealth{
		{Device: "nvme0n1", Brand: "Samsung"},
		{Device: "nvme1n1", Brand: "ADATA"},
		{Device: "sda", Brand: "Seagate"},
	}
	got := diskHealthLabels(health)
	want := []string{"Samsung (0n1)", "ADATA   (1n1)", "Seagate (sda)"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("label[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiskHealthLabelsUseDisplayWidth(t *testing.T) {
	health := []model.DiskHealth{
		{Device: "sda", Brand: "磁盘"},
		{Device: "sdb", Brand: "Disk"},
	}
	labels := diskHealthLabels(health)
	openA := cellIndex(labels[0], "(")
	openB := cellIndex(labels[1], "(")
	if openA != openB {
		t.Fatalf("brand columns differ: %q (%d), %q (%d)", labels[0], openA, labels[1], openB)
	}
}
