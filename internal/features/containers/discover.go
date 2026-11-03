// Package containers resolves published ports with a single inspect per installed engine.
package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Hell077/portguard/internal/features/inspect"
	"github.com/Hell077/portguard/internal/platform/command"
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
