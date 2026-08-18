package collect

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"nasmon/internal/model"
)

type dockerListItem struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
}
type dockerInspect struct {
	RestartCount int `json:"RestartCount"`
	State        struct {
		Health *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
}

func dockerClient() *http.Client {
	tr := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", "/var/run/docker.sock")
	}}
	return &http.Client{Transport: tr, Timeout: 4 * time.Second}
}

func CollectDocker(store *model.Store) {
	c := dockerClient()
	resp, err := c.Get("http://docker/containers/json?all=1")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return
	}
	var items []dockerListItem
	if json.NewDecoder(resp.Body).Decode(&items) != nil {
		return
	}
	out := make([]model.Container, 0, len(items))
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
		out = append(out, co)
	}
	store.Update(func(s *model.Snapshot) { s.Containers = out })
}
