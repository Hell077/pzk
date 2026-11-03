// Package command provides bounded, read-only OS queries. No shell is invoked.
package command

import (
	"context"
	"os/exec"
	"time"
)

func Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	return cmd.Output()
}
