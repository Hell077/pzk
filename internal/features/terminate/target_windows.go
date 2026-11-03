package terminate

import (
	"errors"
	"fmt"
	"github.com/Hell077/pzk/internal/features/inspect"
	"golang.org/x/sys/windows"
	"syscall"
	"unsafe"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var enumWindows = user32.NewProc("EnumWindows")
var windowPID = user32.NewProc("GetWindowThreadProcessId")
var postMessage = user32.NewProc("PostMessageW")

type windowsTarget struct {
	handle  windows.Handle
	pid     uint32
	windows []uintptr
}

func openTarget(p inspect.Process, force bool) (target, error) {
	access := uint32(windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.SYNCHRONIZE)
	if force {
		access |= windows.PROCESS_TERMINATE
	}
	h, err := windows.OpenProcess(access, false, uint32(p.PID))
	if err != nil {
		return nil, fmt.Errorf("open process (permissions): %w", err)
	}
	t := &windowsTarget{handle: h, pid: uint32(p.PID)}
	var created, exit, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(h, &created, &exit, &kernel, &user); err != nil {
		_ = t.Close()
		return nil, err
	}
	if created.Nanoseconds()/1000000 != p.StartedAt {
		_ = t.Close()
		return nil, errors.New("PID was reused; refusing to signal")
	}
	if !force {
		callback := syscall.NewCallback(func(hwnd, extra uintptr) uintptr {
			var pid uint32
			windowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if pid == t.pid {
				t.windows = append(t.windows, hwnd)
			}
			return 1
		})
		if ok, _, err := enumWindows.Call(callback, 0); ok == 0 {
			_ = t.Close()
			return nil, fmt.Errorf("enumerate windows: %w", err)
		}
		if len(t.windows) == 0 {
			_ = t.Close()
			return nil, errors.New("no application window for graceful WM_CLOSE; use the application's shutdown command, or explicitly --force")
		}
	}
	return t, nil
}
func (t *windowsTarget) Signal(force bool) error {
	alive, err := t.Alive()
	if err != nil {
		return err
	}
	if !alive {
		return errors.New("original process exited")
	}
	if force {
		return windows.TerminateProcess(t.handle, 1)
	}
	for _, hwnd := range t.windows {
		var pid uint32
		windowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid != t.pid {
			return errors.New("window owner changed; refusing to close it")
		}
		if ok, _, err := postMessage.Call(hwnd, 0x0010, 0, 0); ok == 0 {
			return fmt.Errorf("WM_CLOSE failed (permissions): %w", err)
		}
	}
	return nil
}
func (t *windowsTarget) Alive() (bool, error) {
	status, err := windows.WaitForSingleObject(t.handle, 0)
	if err != nil {
		return false, err
	}
	return status == uint32(windows.WAIT_TIMEOUT), nil
}
func (t *windowsTarget) Close() error { return windows.CloseHandle(t.handle) }
