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

func TestDockerCompletedDependencyServices(t *testing.T) {
	items := []dockerListItem{
		{
			Labels: map[string]string{
				"com.docker.compose.project":    "vecto_body",
				"com.docker.compose.service":    "backend",
				"com.docker.compose.depends_on": "migrate:service_completed_successfully:false,postgres:service_healthy:false",
			},
		},
	}

	got := dockerCompletedDependencyServices(items)

	if _, ok := got["vecto_body\x00migrate"]; !ok {
		t.Fatal("migrate was not detected as a service_completed_successfully dependency")
	}

	if _, ok := got["vecto_body\x00postgres"]; ok {
		t.Fatal("postgres service_healthy dependency was incorrectly classified as one-shot")
	}
}

func TestDockerIsOneShotFromCompletedDependency(t *testing.T) {
	item := dockerListItem{
		Labels: map[string]string{
			"com.docker.compose.project": "vecto_body",
			"com.docker.compose.service": "migrate",
		},
	}

	completed := map[string]struct{}{
		"vecto_body\x00migrate": {},
	}

	if !dockerIsOneShot(item, completed) {
		t.Fatal("migrate was not classified as one-shot")
	}
}

func TestDockerIsOneShotFromLabels(t *testing.T) {
	tests := []map[string]string{
		{"com.docker.compose.oneoff": "True"},
		{"nasmon.oneshot": "true"},
	}

	for _, labels := range tests {
		if !dockerIsOneShot(dockerListItem{Labels: labels}, nil) {
			t.Fatalf("labels %#v were not classified as one-shot", labels)
		}
	}
}

func TestShouldHideDockerContainer(t *testing.T) {
	tests := []struct {
		name       string
		show       bool
		oneShot    bool
		state      string
		inspectOK  bool
		exitCode   int
		wantHidden bool
	}{
		{"successful completed job", false, true, "exited", true, 0, true},
		{"failed job", false, true, "exited", true, 1, false},
		{"running job", false, true, "running", true, 0, false},
		{"inspect failure", false, true, "exited", false, 0, false},
		{"normal stopped service", false, false, "exited", true, 0, false},
		{"show override", true, true, "exited", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldHideDockerContainer(
				tt.show,
				tt.oneShot,
				tt.state,
				tt.inspectOK,
				tt.exitCode,
			)

			if got != tt.wantHidden {
				t.Fatalf("got hidden=%v, want %v", got, tt.wantHidden)
			}
		})
	}
}
