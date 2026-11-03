package terminate

import (
	"errors"
	"fmt"
	"github.com/Hell077/portguard/internal/features/inspect"
	"golang.org/x/sys/unix"
)

type darwinTarget struct {
	p      inspect.Process
	queue  int
	start  unix.Timeval
	exited bool
}

func openTarget(p inspect.Process, force bool) (target, error) {
	queue, err := unix.Kqueue()
	if err != nil {
		return nil, err
	}
	t := &darwinTarget{p: p, queue: queue}
	change := []unix.Kevent_t{{Ident: uint64(p.PID), Filter: unix.EVFILT_PROC, Flags: unix.EV_ADD | unix.EV_ENABLE, Fflags: unix.NOTE_EXIT}}
	if _, err = unix.Kevent(queue, change, nil, nil); err != nil {
		_ = t.Close()
		return nil, fmt.Errorf("watch process identity (permissions): %w", err)
	}
	if err = verifyIdentity(p); err != nil {
		_ = t.Close()
		return nil, err
	}
	info, err := unix.SysctlKinfoProc("kern.proc.pid", int(p.PID))
	if err != nil {
		_ = t.Close()
		return nil, err
	}
	t.start = info.Proc.P_starttime
	if t.start.Sec*1000+int64(t.start.Usec)/1000 != p.StartedAt {
		_ = t.Close()
		return nil, errors.New("PID identity changed while opening process watcher")
	}
	if err = unix.Kill(int(p.PID), 0); err != nil {
		_ = t.Close()
		return nil, err
	}
	return t, nil
}
func (t *darwinTarget) Signal(force bool) error {
	alive, err := t.Alive()
	if err != nil {
		return err
	}
	if !alive {
		return errors.New("original process exited; refusing to signal")
	}
	info, err := unix.SysctlKinfoProc("kern.proc.pid", int(t.p.PID))
	if err != nil {
		return err
	}
	if info.Proc.P_starttime != t.start {
		return errors.New("PID was reused; refusing to signal")
	}
	// Darwin has no public PID-bound signal API. Keep the final check adjacent to kill.
	s := unix.SIGTERM
	if force {
		s = unix.SIGKILL
	}
	return unix.Kill(int(t.p.PID), s)
}
func (t *darwinTarget) Alive() (bool, error) {
	if t.exited {
		return false, nil
	}
	events := make([]unix.Kevent_t, 1)
	n, err := unix.Kevent(t.queue, nil, events, &unix.Timespec{})
	if n > 0 {
		t.exited = true
		return false, err
	}
	if err != nil {
		return false, err
	}
	info, err := unix.SysctlKinfoProc("kern.proc.pid", int(t.p.PID))
	if err != nil {
		if errors.Is(err, unix.ESRCH) {
			t.exited = true
			return false, nil
		}
		return false, err
	}
	return info.Proc.P_starttime == t.start, nil
}
func (t *darwinTarget) Close() error { return unix.Close(t.queue) }
