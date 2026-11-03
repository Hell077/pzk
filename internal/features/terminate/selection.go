package terminate

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/Hell077/portguard/internal/features/inspect"
	"github.com/Hell077/portguard/internal/features/scan"
)

// Selection is a fixed set of identities to show before interactive confirmation.
// All owners of the port are included, regardless of the UI's current filter.
type Selection struct {
	Port     uint32
	Owners   []inspect.Process
	Warnings []string
}

func nativeBackend(ctx context.Context) backend {
	return backend{snapshot: func(ctx context.Context) (inspect.Snapshot, error) {
		scanCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return scan.Snapshot(scanCtx, scan.Options{Containers: true})
	}, open: openTarget, protected: protectedPIDs(ctx), timeout: 5 * time.Second}
}

// Prepare resolves and validates owners without signaling or opening process handles.
func Prepare(ctx context.Context, port uint32) (Selection, error) {
	b := nativeBackend(ctx)
	s, err := b.snapshot(ctx)
	if err != nil {
		return Selection{}, err
	}
	processes, err := owners(s, port, b.protected)
	if err != nil {
		return Selection{}, err
	}
	return Selection{Port: port, Owners: processes, Warnings: s.Warnings}, nil
}

// RunSelection executes a selection the user has already reviewed and confirmed.
// A fresh scan must exactly match the reviewed identities before handles are opened.
func RunSelection(ctx context.Context, selection Selection, force bool, out io.Writer) error {
	b := nativeBackend(ctx)
	b.reviewed = selection.Owners
	if len(b.reviewed) == 0 {
		return errors.New("no reviewed process identities; refusing to signal")
	}
	return b.run(ctx, selection.Port, force, true, strings.NewReader(""), out)
}
