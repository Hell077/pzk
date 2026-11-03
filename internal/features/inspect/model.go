// Package inspect owns listening-port discovery and its presentation-independent model.
package inspect

import (
	"fmt"
	"github.com/Hell077/pzk/internal/platform/search"
	"net"
	"strconv"
	"strings"
	"time"
)

type Process struct {
	StartedAt  int64  `json:"started_at_ms"`
	PID        int32  `json:"pid"`
	Name       string `json:"name"`
	ParentPID  int32  `json:"parent_pid"`
	ParentName string `json:"parent_name,omitempty"`
	Executable string `json:"executable,omitempty"`
	User       string `json:"user,omitempty"`
}

type Container struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Image         string `json:"image"`
	Engine        string `json:"engine"`
	Service       string `json:"service,omitempty"`
	HostIP        string `json:"host_ip"`
	HostPort      uint32 `json:"host_port"`
	ContainerPort string `json:"container_port"`
}

type Listener struct {
	Protocol   string      `json:"protocol"`
	Address    string      `json:"address"`
	Port       uint32      `json:"port"`
	Process    Process     `json:"process"`
	Service    string      `json:"service,omitempty"`
	Containers []Container `json:"containers,omitempty"`
}

func (l Listener) Endpoint() string { return net.JoinHostPort(l.Address, strconv.Itoa(int(l.Port))) }
func (l Listener) Exposure() string {
	ip := net.ParseIP(l.Address)
	if ip != nil && ip.IsLoopback() {
		return "loopback"
	}
	if ip == nil || ip.IsUnspecified() {
		return "all interfaces"
	}
	return "interface-bound"
}
func (l Listener) Reason() string {
	if len(l.Containers) > 0 {
		c := l.Containers[0]
		return fmt.Sprintf("%s publishes %s → %s (%s)", c.Engine, l.Endpoint(), c.ContainerPort, c.Name)
	}
	name := l.Process.Name
	if name == "" {
		name = "owner unavailable"
	}
	if l.Service != "" {
		return fmt.Sprintf("%s owns this %s socket; managed by %s", name, l.Protocol, l.Service)
	}
	return fmt.Sprintf("%s owns this %s socket", name, l.Protocol)
}

type Snapshot struct {
	Listeners Listeners     `json:"listeners"`
	Warnings  []string      `json:"warnings"`
	ScannedAt time.Time     `json:"scanned_at"`
	Duration  time.Duration `json:"duration_ns"`
}

type Listeners []Listener

// Project uses Go 1.27 generic methods to derive typed view data without coupling discovery to a UI.
func (ls Listeners) Project[T any](fn func(Listener) T) []T {
	out := make([]T, 0, len(ls))
	for _, l := range ls {
		out = append(out, fn(l))
	}
	return out
}

func (ls Listeners) Filter(port uint32, protocol, query string) Listeners {
	out := make(Listeners, 0, len(ls))
	query = strings.ToLower(query)
	for _, l := range ls {
		if port != 0 && l.Port != port || protocol != "all" && l.Protocol != protocol {
			continue
		}
		if query != "" {
			text := fmt.Sprintf("%d %s %s %s %d %s %s", l.Port, l.Address, l.Protocol, l.Process.Name, l.Process.PID, l.Process.ParentName, l.Service)
			for _, c := range l.Containers {
				text += " " + c.Name + " " + c.Image + " " + c.Service
			}
			if !search.Contains(strings.ToLower(text), query) {
				continue
			}
		}
		out = append(out, l)
	}
	return out
}
