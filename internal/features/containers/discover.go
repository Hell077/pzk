// Package containers resolves published ports with a single inspect per installed engine.
package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Hell077/pzk/internal/features/inspect"
	"github.com/Hell077/pzk/internal/platform/command"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type containerInfo struct {
	ID        string `json:"Id"`
	Name      string
	ImageName string
	Config    struct {
		Image  string
		Labels map[string]string
	}
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string
		}
	}
}

func Discover(ctx context.Context) ([]inspect.Container, []string) {
	var ports []inspect.Container
	var warnings []string
	for _, engine := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(engine); err != nil {
			continue
		}
		if engine == "docker" {
			endpoint := os.Getenv("DOCKER_HOST")
			if endpoint == "" || os.Getenv("DOCKER_CONTEXT") != "" {
				data, err := command.Output(ctx, "docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
				if err != nil {
					warnings = append(warnings, "docker: could not verify a local context; mappings skipped")
					continue
				}
				endpoint = strings.TrimSpace(string(data))
			}
			if !LocalEndpoint(endpoint) {
				warnings = append(warnings, "docker: remote context skipped; only local ports are inspected")
				continue
			}
		} else if host := os.Getenv("CONTAINER_HOST"); host != "" && !LocalEndpoint(host) {
			warnings = append(warnings, "podman: remote CONTAINER_HOST skipped")
			continue
		} else if os.Getenv("CONTAINER_CONNECTION") != "" {
			warnings = append(warnings, "podman: explicit remote connection skipped")
			continue
		}
		ids, err := command.Output(ctx, engine, "ps", "-q")
		if err != nil {
			warnings = append(warnings, engine+": container context unavailable (daemon, permissions, or timeout)")
			continue
		}
		args := append([]string{"inspect"}, strings.Fields(string(ids))...)
		if len(args) == 1 {
			continue
		}
		data, err := command.Output(ctx, engine, args...)
		if err != nil {
			warnings = append(warnings, engine+": inspect failed")
			continue
		}
		parsed, err := Parse(engine, data)
		if err != nil {
			warnings = append(warnings, engine+": invalid inspect response")
			continue
		}
		ports = append(ports, parsed...)
	}
	return ports, warnings
}

func Parse(engine string, data []byte) ([]inspect.Container, error) {
	var infos []containerInfo
	if err := json.Unmarshal(data, &infos); err != nil {
		return nil, err
	}
	var result []inspect.Container
	for _, c := range infos {
		for target, bindings := range c.NetworkSettings.Ports {
			for _, b := range bindings {
				p, err := strconv.ParseUint(b.HostPort, 10, 16)
				if err != nil || p == 0 {
					return nil, fmt.Errorf("invalid published port %q", b.HostPort)
				}
				image := c.Config.Image
				if image == "" {
					image = c.ImageName
				}
				service := c.Config.Labels["com.docker.compose.service"]
				if project := c.Config.Labels["com.docker.compose.project"]; project != "" && service != "" {
					service = project + "/" + service
				}
				result = append(result, inspect.Container{ID: c.ID, Name: strings.TrimPrefix(c.Name, "/"), Image: image, Engine: engine, Service: service, HostIP: b.HostIP, HostPort: uint32(p), ContainerPort: target})
			}
		}
	}
	return result, nil
}

// LocalEndpoint accepts only local IPC transports, never TCP or SSH endpoints.
func LocalEndpoint(endpoint string) bool {
	return strings.HasPrefix(endpoint, "unix://") || strings.HasPrefix(endpoint, "npipe://")
}
