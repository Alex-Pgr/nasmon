package collect

import (
	"reflect"
	"testing"

	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

func TestSortDiskUsageUsesConfiguredPriorityThenAlphabetical(t *testing.T) {
	usage := []model.DiskUsage{
		{Path: "/mnt/zeta"},
		{Path: "/srv/archive"},
		{Path: "/"},
		{Path: "/mnt/alpha"},
		{Path: "/srv/data"},
	}
	priority := map[string]int{
		"/":            0,
		"/srv/data":    1,
		"/srv/archive": 2,
	}

	sortDiskUsage(usage, priority)
	got := make([]string, len(usage))
	for i := range usage {
		got[i] = usage[i].Path
	}
	want := []string{"/", "/srv/data", "/srv/archive", "/mnt/alpha", "/mnt/zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}
