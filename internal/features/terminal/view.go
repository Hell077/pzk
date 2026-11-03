package terminal

import (
	"context"
	"fmt"
	"github.com/Hell077/pzk/internal/features/inspect"
	"github.com/Hell077/pzk/internal/features/scan"
	"strings"
	"time"
)

type options struct {
	port                     uint
	low, high                uint32
	protocol, query          string
	json, once, noContainers bool
	interval                 time.Duration
}

func (o options) filter(ls inspect.Listeners) inspect.Listeners {
	ls = ls.Filter(uint32(o.port), o.protocol, o.query)
	if o.low == 0 {
		return ls
	}
	out := make(inspect.Listeners, 0, len(ls))
	for _, l := range ls {
		if l.Port >= o.low && l.Port <= o.high {
			out = append(out, l)
		}
	}
	return out
}

func load(ctx context.Context, o options) (inspect.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return scan.Snapshot(ctx, scan.Options{Containers: !o.noContainers})
}

// Safe strips control sequences from untrusted OS metadata before rendering.
func Safe(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r >= 127 && r <= 159 {
			return -1
		}
		return r
	}, s)
}

func Plain(s inspect.Snapshot) string {
	var b strings.Builder
	fmt.Fprintln(&b, "PZK · port inspection")
	fmt.Fprintln(&b, "PROTO  ENDPOINT                  PID     PROCESS              PARENT / SERVICE / CONTAINER")
	rows := s.Listeners.Project(func(l inspect.Listener) string {
		context := l.Process.ParentName
		if l.Service != "" {
			context += " / " + l.Service
		}
		for _, c := range l.Containers {
			context += " / " + c.Engine + ":" + c.Name
			if c.Service != "" {
				context += " (" + c.Service + ")"
			}
		}
		name := l.Process.Name
		if name == "" {
			name = "unknown"
		}
		return fmt.Sprintf("%-6s %-25s %-7d %-20s %s\n", strings.ToUpper(l.Protocol), l.Endpoint(), l.Process.PID, Safe(name), Safe(context))
	})
	for _, row := range rows {
		b.WriteString(row)
	}
	if len(rows) == 0 {
		fmt.Fprintln(&b, "No matching listeners or published container ports visible to this user.")
	}
	for _, w := range s.Warnings {
		fmt.Fprintln(&b, "!", Safe(w))
	}
	fmt.Fprintln(&b, "Review the owner and its service/container before changing exposure or restarting it.")
	return b.String()
}

func Detail(s inspect.Snapshot) string {
	var b strings.Builder
	b.WriteString(Plain(s))
	for _, l := range s.Listeners {
		fmt.Fprintf(&b, "\n%s %s\n  %s\n  Parent: %s (PID %d)\n  User: %s\n  Executable: %s\n  Service: %s\n  Exposure: %s\n", strings.ToUpper(l.Protocol), l.Endpoint(), Safe(l.Reason()), Safe(l.Process.ParentName), l.Process.ParentPID, Safe(l.Process.User), Safe(l.Process.Executable), Safe(l.Service), l.Exposure())
		for _, c := range l.Containers {
			fmt.Fprintf(&b, "  Container: %s / %s (%s), service %s, target %s\n", Safe(c.Engine), Safe(c.Name), Safe(c.Image), Safe(c.Service), Safe(c.ContainerPort))
		}
	}
	return b.String()
}
