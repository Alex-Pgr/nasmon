package collect

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Alex-Pgr/nas_monitoring/internal/model"
)

type dockerListItem struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Labels map[string]string `json:"Labels"`
}

type dockerInspect struct {
	RestartCount int `json:"RestartCount"`
	State        struct {
		Health *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
}

type dockerStats struct {
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
}

var dockerHTTPClient = newDockerClient()

func newDockerClient() *http.Client {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", "/var/run/docker.sock")
		},
		MaxIdleConns:        4,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     90 * time.Second,
	}
	return &http.Client{Transport: tr, Timeout: 4 * time.Second}
}

// Match Docker CLI's Linux memory display: usage minus reclaimable inactive
// file cache. cgroup v1 exposes total_inactive_file, cgroup v2 inactive_file.
func dockerMemoryUsage(st dockerStats) uint64 {
	usage := st.MemoryStats.Usage
	for _, key := range []string{"total_inactive_file", "inactive_file"} {
		if cache, ok := st.MemoryStats.Stats[key]; ok && cache < usage {
			return usage - cache
		}
	}
	return usage
}

func collectDockerMemory(c *http.Client, id string) (uint64, bool) {
	resp, err := c.Get("http://docker/containers/" + id + "/stats?stream=false")
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return 0, false
	}
	var st dockerStats
	if json.NewDecoder(resp.Body).Decode(&st) != nil {
		return 0, false
	}
	return dockerMemoryUsage(st), true
}

func collectDockerInspect(c *http.Client, id string, co *model.Container) {
	resp, err := c.Get("http://docker/containers/" + id + "/json")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return
	}
	var ins dockerInspect
	if json.NewDecoder(resp.Body).Decode(&ins) != nil {
		return
	}
	co.Restarts = ins.RestartCount
	if ins.State.Health != nil {
		co.Health = ins.State.Health.Status
	}
}

func CollectDocker(store *model.Store) bool {
	c := dockerHTTPClient
	resp, err := c.Get("http://docker/containers/json?all=1")
	if err != nil {
		return false
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return false
	}
	var items []dockerListItem
	decodeErr := json.NewDecoder(resp.Body).Decode(&items)
	resp.Body.Close()
	if decodeErr != nil {
		return false
	}

	type collectedContainer struct {
		container model.Container
		group     string
	}
	collected := make([]collectedContainer, 0, len(items))
	for _, it := range items {
		name := ""
		if len(it.Names) > 0 {
			name = strings.TrimPrefix(it.Names[0], "/")
		}
		group := it.Labels["com.docker.compose.project"]
		if group == "" {
			group = name
		}
		collected = append(collected, collectedContainer{
			container: model.Container{ID: it.ID, Name: name, Status: it.Status, State: it.State},
			group:     group,
		})
	}

	// Keep all per-container Docker API work behind one bounded pool. Each
	// worker inspects a container and then, for running containers, samples its
	// memory. This prevents sequential inspect latency from growing linearly
	// with large stacks while avoiding an unbounded request burst.
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i := range collected {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			co := &collected[i].container
			collectDockerInspect(c, co.ID, co)
			if co.State == "running" {
				if memory, ok := collectDockerMemory(c, co.ID); ok {
					co.MemoryBytes = memory
				}
			}
		}(i)
	}
	wg.Wait()

	sort.SliceStable(collected, func(i, j int) bool {
		gi := strings.ToLower(collected[i].group)
		gj := strings.ToLower(collected[j].group)
		if gi != gj {
			return gi < gj
		}
		return strings.ToLower(collected[i].container.Name) < strings.ToLower(collected[j].container.Name)
	})

	out := make([]model.Container, 0, len(collected))
	for _, item := range collected {
		out = append(out, item.container)
	}
	store.Update(func(s *model.Snapshot) { s.Containers = out })
	return true
}
