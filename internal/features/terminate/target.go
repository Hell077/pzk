package terminate

import (
	"errors"
	"fmt"
	"github.com/Hell077/pzk/internal/features/inspect"
	"github.com/shirou/gopsutil/v4/process"
)

type target interface {
	Signal(bool) error
	Alive() (bool, error)
	Close() error
}

func verifyIdentity(p inspect.Process) error {
	current, err := process.NewProcess(p.PID)
	if err != nil {
		return fmt.Errorf("process disappeared or cannot be queried: %w", err)
	}
	started, err := current.CreateTime()
	if err != nil {
		return fmt.Errorf("cannot verify process creation time (permissions): %w", err)
	}
	if started != p.StartedAt {
		return errors.New("PID was reused or process identity changed; refusing to signal")
	}
	return nil
}
