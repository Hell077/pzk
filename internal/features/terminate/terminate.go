// Package terminate implements explicit, confirmed, identity-checked process termination.
package terminate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Hell077/portguard/internal/features/inspect"
	"github.com/Hell077/portguard/internal/features/scan"
	"github.com/shirou/gopsutil/v4/process"
)

type backend struct {
	snapshot  func(context.Context) (inspect.Snapshot, error)
	open      func(inspect.Process, bool) (target, error)
	protected map[int32]bool
	timeout   time.Duration
}

func Run(ctx context.Context, port uint32, force, yes bool, in io.Reader, out io.Writer) error {
	b := backend{snapshot: func(ctx context.Context) (inspect.Snapshot, error) {
		return scan.Snapshot(ctx, scan.Options{Containers: true})
	}, open: openTarget, protected: protectedPIDs(ctx), timeout: 5 * time.Second}
	return b.run(ctx, port, force, yes, in, out)
}

func protectedPIDs(ctx context.Context) map[int32]bool {
	pids := map[int32]bool{0: true, 1: true, int32(os.Getpid()): true}
	pid := int32(os.Getppid())
	for pid > 0 && !pids[pid] {
		pids[pid] = true
		p, err := process.NewProcessWithContext(ctx, pid)
		if err != nil {
			break
		}
		pid, err = p.PpidWithContext(ctx)
		if err != nil {
			break
		}
	}
	return pids
}

func owners(s inspect.Snapshot, port uint32, protected map[int32]bool) ([]inspect.Process, error) {
	found := map[int32]inspect.Process{}
	for _, l := range s.Listeners {
		if l.Port != port {
			continue
		}
		if len(l.Containers) > 0 {
			return nil, errors.New("port has a container publication; stop the named container with its engine instead of killing a host proxy")
		}
		p := l.Process
		name := strings.ToLower(p.Name)
		for _, proxy := range []string{"docker-proxy", "com.docker.backend", "rootlessport", "rootlesskit", "vpnkit", "gvproxy"} {
			if strings.Contains(name, proxy) {
				return nil, fmt.Errorf("refusing container runtime/proxy %s; stop its container with the engine", p.Name)
			}
		}
		if p.PID <= 0 || p.Name == "" || p.StartedAt <= 0 {
			return nil, errors.New("cannot identify every port owner; check permissions and retry; no process was signaled")
		}
		if protected[p.PID] {
			return nil, fmt.Errorf("refusing protected PID %d (%s): system, self, or an ancestor", p.PID, p.Name)
		}
		if old, ok := found[p.PID]; ok && old.StartedAt != p.StartedAt {
			return nil, errors.New("process identity changed during scan; retry")
		}
		found[p.PID] = p
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no visible owner for port %d; it may be free or hidden by permissions", port)
	}
	result := make([]inspect.Process, 0, len(found))
	for _, p := range found {
		result = append(result, p)
	}
	slices.SortFunc(result, func(a, b inspect.Process) int {
		if a.PID < b.PID {
			return -1
		}
		if a.PID > b.PID {
			return 1
		}
		return 0
	})
	return result, nil
}

func sameOwners(a, b []inspect.Process) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].PID != b[i].PID || a[i].StartedAt != b[i].StartedAt {
			return false
		}
	}
	return true
}
func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r >= 127 && r <= 159 {
			return -1
		}
		return r
	}, s)
}

func (b backend) run(ctx context.Context, port uint32, force, yes bool, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	s, err := b.snapshot(ctx)
	if err != nil {
		return err
	}
	expected, err := owners(s, port, b.protected)
	if err != nil {
		return err
	}
	action := "Request graceful shutdown"
	if force {
		action = "FORCE termination (unsaved work may be lost)"
	}
	fmt.Fprintf(out, "%s for ALL %d owners of TCP/UDP port %d:\n", action, len(expected), port)
	for _, p := range expected {
		fmt.Fprintf(out, "  PID %d  %s  parent %s (%d)\n", p.PID, safe(p.Name), safe(p.ParentName), p.ParentPID)
	}
	fmt.Fprintln(out, "This affects each process and every port it owns. A service manager may restart it.")
	for _, w := range s.Warnings {
		fmt.Fprintln(out, "!", safe(w))
	}
	if !yes {
		fmt.Fprint(out, "Type yes to continue: ")
		// A pipe must provide the complete word too; EOF and all other responses cancel safely.
		type response struct {
			line string
			err  error
		}
		reply := make(chan response, 1)
		go func() { line, err := bufio.NewReader(in).ReadString('\n'); reply <- response{line, err} }()
		select {
		case <-ctx.Done():
			return fmt.Errorf("confirmation cancelled: %w; no process was signaled", ctx.Err())
		case r := <-reply:
			if r.err != nil && !errors.Is(r.err, io.EOF) {
				return fmt.Errorf("read confirmation: %w", r.err)
			}
			if strings.TrimSpace(r.line) != "yes" {
				return errors.New("cancelled; no process was signaled")
			}
		}
	}
	var handles []target
	defer func() {
		for _, h := range handles {
			_ = h.Close()
		}
	}()
	// Open and validate every handle before mutating any process.
	for _, p := range expected {
		h, err := b.open(p, force)
		if err != nil {
			return fmt.Errorf("prepare PID %d: %w; no process was signaled", p.PID, err)
		}
		handles = append(handles, h)
	}
	current, err := b.snapshot(ctx)
	if err != nil {
		return fmt.Errorf("recheck port ownership: %w; no process was signaled", err)
	}
	actual, err := owners(current, port, b.protected)
	if err != nil {
		return err
	}
	if !sameOwners(expected, actual) {
		return errors.New("port owners changed after confirmation; retry; no process was signaled")
	}
	for i, h := range handles {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("operation interrupted after %d signals: %w", i, err)
		}
		if err := h.Signal(force); err != nil {
			return fmt.Errorf("signal PID %d: %w (%d earlier requests sent; no automatic force escalation)", expected[i].PID, err, i)
		}
		fmt.Fprintf(out, "Shutdown request sent to PID %d.\n", expected[i].PID)
	}
	// Verify process exit, not just signal delivery. Never silently escalate after timeout.
	deadline := time.NewTimer(b.timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		var alive []int32
		for i, h := range handles {
			running, err := h.Alive()
			if err != nil {
				return fmt.Errorf("verify PID %d: %w", expected[i].PID, err)
			}
			if running {
				alive = append(alive, expected[i].PID)
			}
		}
		if len(alive) == 0 {
			after, err := b.snapshot(ctx)
			if err != nil {
				return fmt.Errorf("targets exited; final port check failed: %w", err)
			}
			for _, l := range after.Listeners {
				if l.Port == port {
					return fmt.Errorf("original targets exited, but port %d is occupied again; inspect it before retrying", port)
				}
			}
			fmt.Fprintf(out, "All target processes exited; port %d has no visible owner.\n", port)
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for shutdown: %w", ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("shutdown timeout: PIDs %v remain alive; no force escalation was performed", alive)
		case <-ticker.C:
		}
	}
}
