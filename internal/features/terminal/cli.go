// Package terminal owns commands, machine output, and the interactive terminal feature.
package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Hell077/portguard/internal/features/terminate"
	"github.com/Hell077/portguard/internal/platform/search"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

func Run(args []string, version string, out, errOut io.Writer) error {
	return run(args, version, os.Stdin, out, errOut)
}

func run(args []string, version string, in io.Reader, out, errOut io.Writer) error {
	command := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	if command != "" && command != "list" && command != "inspect" && command != "watch" && command != "kill" {
		return fmt.Errorf("unknown command %q; use portguard --help", command)
	}
	fs := flag.NewFlagSet("portguard "+command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	var o options
	var showVersion, force, yes bool
	var portRange, portArg string
	o.protocol = "all"
	o.interval = 2 * time.Second
	if command == "kill" {
		fs.BoolVar(&force, "force", false, "force termination; may lose unsaved work")
		fs.BoolVar(&yes, "yes", false, "skip confirmation (identity and ownership checks still apply)")
	} else {
		fs.BoolVar(&o.json, "json", false, "JSON snapshot (watch: one object per refresh)")
		fs.BoolVar(&o.noContainers, "no-containers", false, "skip Docker and Podman queries")
		if command == "list" || command == "" {
			fs.StringVar(&portRange, "range", "", "inclusive port range, e.g. 3000-9000")
			fs.StringVar(&o.protocol, "protocol", "all", "all, tcp, or udp")
			fs.StringVar(&o.query, "filter", "", "filter owner, parent, service, address, or container")
		}
		if command == "" {
			fs.UintVar(&o.port, "port", 0, "show a single port (1–65535)")
			fs.BoolVar(&o.once, "once", false, "one plain-text snapshot")
		}
		if command == "" || command == "watch" {
			fs.DurationVar(&o.interval, "interval", 2*time.Second, "refresh interval (at least 500ms)")
		}
	}
	if command == "" {
		fs.BoolVar(&showVersion, "version", false, "print installed version")
	}
	fs.Usage = func() {
		if command == "" {
			fmt.Fprint(errOut, "Portguard — know what owns your ports.\n\nUsage: portguard <command> [flags]\n\n  list [--range 3000-9000] [--json]\n  inspect PORT [--json]\n  watch PORT [--interval 2s]\n  kill PORT [--force] [--yes]\n\nNo command opens the live terminal UI. Inspection never changes processes or firewalls.\nKill asks for confirmation unless --yes; it never escalates automatically.\n")
		} else {
			fmt.Fprintf(errOut, "Usage: portguard %s", command)
			if command != "list" {
				fmt.Fprint(errOut, " PORT")
			}
			fmt.Fprintln(errOut, " [flags]")
		}
		fs.PrintDefaults()
	}
	// Accept flags both after the positional port and before it.
	if command == "inspect" || command == "watch" || command == "kill" {
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			portArg = args[0]
			args = args[1:]
		}
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := fs.Args()
	if portArg == "" && (command == "inspect" || command == "watch" || command == "kill") && len(rest) > 0 {
		portArg = rest[0]
		rest = rest[1:]
		if len(rest) > 0 {
			return errors.New("place flags before the port or directly after it")
		}
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected argument %q; use --help", rest[0])
	}
	if showVersion {
		fmt.Fprintf(out, "portguard %s (%s; search: %s)\n", version, runtime.Version(), search.Backend)
		return nil
	}
	if command == "inspect" || command == "watch" || command == "kill" {
		p, err := parsePort(portArg)
		if err != nil {
			return err
		}
		o.port = uint(p)
	}
	if o.port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if portRange != "" {
		var err error
		o.low, o.high, err = parseRange(portRange)
		if err != nil {
			return err
		}
	}
	if o.protocol != "all" && o.protocol != "tcp" && o.protocol != "udp" {
		return errors.New("protocol must be all, tcp, or udp")
	}
	if o.interval < 500*time.Millisecond {
		return errors.New("interval must be at least 500ms")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if command == "kill" {
		return terminate.Run(ctx, uint32(o.port), force, yes, in, out)
	}
	tty := isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stdin.Fd()) && os.Getenv("TERM") != "dumb"
	if (command == "" && !o.json && !o.once || command == "watch" && !o.json) && tty {
		_, err := tea.NewProgram(newModel(ctx, o), tea.WithContext(ctx), tea.WithOutput(out), tea.WithAltScreen()).Run()
		if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
			return nil
		}
		return err
	}
	for {
		snapshot, err := load(ctx, o)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		snapshot.Listeners = o.filter(snapshot.Listeners)
		if command == "inspect" && len(snapshot.Listeners) == 0 {
			return fmt.Errorf("no visible owner or container mapping for port %d; check permissions", o.port)
		}
		if o.json {
			e := json.NewEncoder(out)
			if command != "watch" {
				e.SetIndent("", "  ")
			}
			if err = e.Encode(snapshot); err != nil {
				return err
			}
		} else {
			body := Plain(snapshot)
			if command == "inspect" {
				body = Detail(snapshot)
			}
			if _, err = fmt.Fprint(out, body); err != nil {
				return err
			}
		}
		if command != "watch" {
			return nil
		}
		timer := time.NewTimer(o.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func parsePort(s string) (uint32, error) {
	p, err := strconv.ParseUint(s, 10, 16)
	if err != nil || p == 0 {
		return 0, fmt.Errorf("invalid port %q: expected 1–65535", s)
	}
	return uint32(p), nil
}
func parseRange(s string) (uint32, uint32, error) {
	a, b, ok := strings.Cut(s, "-")
	if !ok {
		return 0, 0, fmt.Errorf("invalid range %q: expected START-END", s)
	}
	low, e1 := parsePort(a)
	high, e2 := parsePort(b)
	if e1 != nil || e2 != nil || low > high {
		return 0, 0, fmt.Errorf("invalid range %q: expected 1 <= START <= END <= 65535", s)
	}
	return low, high, nil
}
