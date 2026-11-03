package terminate

import (
	"fmt"
	"github.com/Hell077/portguard/internal/features/inspect"
	"golang.org/x/sys/unix"
)

type linuxTarget struct{ fd int }

func openTarget(p inspect.Process, force bool) (target, error) {
	fd, err := unix.PidfdOpen(int(p.PID), 0)
	if err != nil {
		return nil, fmt.Errorf("open PID-bound handle (Linux 5.3+ required; no unsafe PID fallback): %w", err)
	}
	t := &linuxTarget{fd: fd}
	if err := verifyIdentity(p); err != nil {
		_ = t.Close()
		return nil, err
	}
	// Permission preflight without delivering a signal.
	if err := unix.PidfdSendSignal(fd, 0, nil, 0); err != nil {
		_ = t.Close()
		return nil, fmt.Errorf("permission or process state: %w", err)
	}
	return t, nil
}
func (t *linuxTarget) Signal(force bool) error {
	s := unix.SIGTERM
	if force {
		s = unix.SIGKILL
	}
	return unix.PidfdSendSignal(t.fd, s, nil, 0)
}
func (t *linuxTarget) Alive() (bool, error) {
	fds := []unix.PollFd{{Fd: int32(t.fd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, 0)
	return n == 0, err
}
func (t *linuxTarget) Close() error { return unix.Close(t.fd) }
