package collect

import "testing"

func TestDockerMemoryUsageSubtractsInactiveFile(t *testing.T) {
	var st dockerStats
	st.MemoryStats.Usage = 1024
	st.MemoryStats.Stats = map[string]uint64{"inactive_file": 256}
	if got := dockerMemoryUsage(st); got != 768 {
		t.Fatalf("dockerMemoryUsage() = %d, want 768", got)
	}
}

func TestDockerMemoryUsageFallsBackToUsage(t *testing.T) {
	var st dockerStats
	st.MemoryStats.Usage = 1024
	st.MemoryStats.Stats = map[string]uint64{"inactive_file": 2048}
	if got := dockerMemoryUsage(st); got != 1024 {
		t.Fatalf("dockerMemoryUsage() = %d, want 1024", got)
	}
}
