//go:build !linux && !darwin && !windows

package terminate

import (
	"errors"
	"github.com/Hell077/pzk/internal/features/inspect"
)

func openTarget(p inspect.Process, force bool) (target, error) {
	return nil, errors.New("safe termination is supported on Linux, macOS, and Windows")
}
