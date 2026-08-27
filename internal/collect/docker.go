package collect

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"nasmon/internal/model"
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

var dockerHTTPClient = newDockerClient()

func newDockerClient() *http.Client {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", "/var/run/docker.sock")
		},
		MaxIdleConns:        2,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     90 * time.Second,
	}
	return &http.Client{Transport: tr, Timeout: 4 * time.Second}
}

func CollectDocker(store *model.Store) {
	c := dockerHTTPClient
	resp, err := c.Get("http://docker/containers/json?all=1")
	if err != nil {
		return
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return
	}
	var items []dockerListItem
	decodeErr := json.NewDecoder(resp.Body).Decode(&items)
	resp.Body.Close()
	if decodeErr != nil {
		return
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
		co := model.Container{ID: it.ID, Name: name, Status: it.Status, State: it.State}
		r, err := c.Get("http://docker/containers/" + it.ID + "/json")
		if err == nil {
			var ins dockerInspect
			if json.NewDecoder(r.Body).Decode(&ins) == nil {
				co.Restarts = ins.RestartCount
				if ins.State.Health != nil {
					co.Health = ins.State.Health.Status
				}
			}
			r.Body.Close()
		}

		group := it.Labels["com.docker.compose.project"]
		if group == "" {
			// Non-Compose containers still get a deterministic alphabetical order.
			group = name
		}
		collected = append(collected, collectedContainer{container: co, group: group})
	}

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
}
