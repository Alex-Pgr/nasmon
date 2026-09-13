package collect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPhysicalBlockAtResolvesMMCPPartition(t *testing.T) {
	root := t.TempDir()
	sysBlock := filepath.Join(root, "sys", "class", "block")
	targetParent := filepath.Join(root, "devices", "platform", "soc", "mmc_host", "mmc0", "block", "mmcblk0")
	targetPartition := filepath.Join(targetParent, "mmcblk0p2")
	if err := os.MkdirAll(targetPartition, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetPartition, "partition"), []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sysBlock, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPartition, filepath.Join(sysBlock, "mmcblk0p2")); err != nil {
		t.Fatal(err)
	}

	if got := physicalBlockAt(sysBlock, "/dev/mmcblk0p2"); got != "mmcblk0" {
		t.Fatalf("physicalBlockAt = %q, want mmcblk0", got)
	}
}

func TestSmartCapableCandidateExcludesMMC(t *testing.T) {
	for _, dev := range []string{"mmcblk0", "mmcblk1"} {
		if smartCapableCandidate(dev) {
			t.Fatalf("%s should not be a SMART candidate", dev)
		}
	}
	for _, dev := range []string{"sda", "nvme0n1"} {
		if !smartCapableCandidate(dev) {
			t.Fatalf("%s should remain a SMART candidate", dev)
		}
	}
}
