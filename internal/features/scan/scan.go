// Package scan orchestrates socket, process, container, and service discovery.
package scan

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Hell077/portguard/internal/features/containers"
	"github.com/Hell077/portguard/internal/features/inspect"
	"github.com/Hell077/portguard/internal/features/services"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type Options struct{ Containers bool }

func Snapshot(ctx context.Context, opts Options) (inspect.Snapshot, error) {
	start := time.Now()
	snapshot := inspect.Snapshot{ScannedAt: start, Listeners: make(inspect.Listeners, 0), Warnings: make([]string, 0)}
	connections, err := gnet.ConnectionsWithContext(ctx, "all")
	if err != nil {
		return snapshot, fmt.Errorf("read local sockets: %w (check OS permissions and, on macOS, lsof availability)", err)
	}
	seen := make(map[string]bool)
	pids := make(map[int32]bool)
	for _, c := range connections {
		if c.Laddr.Port == 0 {
			continue
		}
		protocol := "tcp"
		if c.Type == 2 {
			protocol = "udp"
		} else if c.Type != 1 || !strings.EqualFold(c.Status, "LISTEN") {
			continue
		}
		l := inspect.Listener{Protocol: protocol, Address: c.Laddr.IP, Port: c.Laddr.Port, Process: inspect.Process{PID: c.Pid}}
		key := fmt.Sprintf("%s/%s/%d", protocol, l.Endpoint(), c.Pid)
		if seen[key] {
			continue
		}
		seen[key] = true
		snapshot.Listeners = append(snapshot.Listeners, l)
		if c.Pid > 0 {
			pids[c.Pid] = true
		}
	}
	// Bound process metadata work; resolve each owning PID only once per scan.
	processes := make(map[int32]inspect.Process)
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 8)
	for pid := range pids {
		wg.Go(func() {
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()
			info := readProcess(ctx, pid)
			mu.Lock()
			processes[pid] = info
			mu.Unlock()
		})
	}
	wg.Wait()
	ids := make([]int32, 0, len(processes))
	for pid, info := range processes {
		ids = append(ids, pid)
		if info.ParentPID > 0 {
			ids = append(ids, info.ParentPID)
		}
	}
	serviceMap, warnings := services.Discover(ctx, ids)
	snapshot.Warnings = append(snapshot.Warnings, warnings...)
	missing := false
	for i := range snapshot.Listeners {
		l := &snapshot.Listeners[i]
		if info, ok := processes[l.Process.PID]; ok {
			l.Process = info
		}
		l.Service = serviceMap[l.Process.PID]
		if l.Process.PID == 0 || l.Process.Name == "" {
			missing = true
		}
	}
	if missing {
		snapshot.Warnings = append(snapshot.Warnings, "Some owners are unavailable: permissions or a process exiting during the scan. Elevate only if you trust this binary.")
	}
	if opts.Containers {
		ports, warnings := containers.Discover(ctx)
		snapshot.Warnings = append(snapshot.Warnings, warnings...)
		snapshot.Listeners = MergeContainers(snapshot.Listeners, ports)
	}
	slices.SortFunc(snapshot.Listeners, func(a, b inspect.Listener) int {
		if n := cmp.Compare(a.Port, b.Port); n != 0 {
			return n
		}
		if n := cmp.Compare(a.Protocol, b.Protocol); n != 0 {
			return n
		}
		if n := cmp.Compare(a.Address, b.Address); n != 0 {
			return n
		}
		return cmp.Compare(a.Process.PID, b.Process.PID)
	})
	snapshot.Duration = time.Since(start)
	if err := ctx.Err(); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func readProcess(ctx context.Context, pid int32) inspect.Process {
	info := inspect.Process{PID: pid}
	p, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return info
	}
	info.StartedAt, _ = p.CreateTimeWithContext(ctx)
	info.Name, _ = p.NameWithContext(ctx)
	info.Executable, _ = p.ExeWithContext(ctx)
	info.User, _ = p.UsernameWithContext(ctx)
	info.ParentPID, _ = p.PpidWithContext(ctx)
	if info.ParentPID > 0 {
		if parent, err := process.NewProcessWithContext(ctx, info.ParentPID); err == nil {
			info.ParentName, _ = parent.NameWithContext(ctx)
		}
	}
	return info
}

// MergeContainers preserves published mappings even when the engine uses a VM or NAT
// and no user-space host listener exists. A PID of zero explicitly means unknown.
func MergeContainers(listeners inspect.Listeners, ports []inspect.Container) inspect.Listeners {
	for _, c := range ports {
		_, protocol, ok := strings.Cut(c.ContainerPort, "/")
		if !ok {
			continue
		}
		found := false
		host := c.HostIP
		if host == "" {
			host = "0.0.0.0"
		}
		for i := range listeners {
			l := &listeners[i]
			if l.Port == c.HostPort && l.Protocol == protocol && l.Address == host {
				l.Containers = append(l.Containers, c)
				found = true
			}
		}
		if !found {
			listeners = append(listeners, inspect.Listener{Protocol: protocol, Address: host, Port: c.HostPort, Containers: []inspect.Container{c}})
		}
	}
	return listeners
}
