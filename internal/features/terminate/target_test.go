//go:build linux || darwin || windows

package terminate

import (
	"bufio"
	"fmt"
	"github.com/Hell077/portguard/internal/features/inspect"
	"github.com/shirou/gopsutil/v4/process"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// The only real process signaled by tests is this test-owned child.
func TestProcessHelper(t *testing.T) {
	if os.Getenv("PORTGUARD_TEST_CHILD") != "1" {
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	fmt.Println(ln.Addr().(*net.TCPAddr).Port)
	for {
		c, err := ln.Accept()
		if err != nil {
			os.Exit(0)
		}
		c.Close()
	}
}
func TestBoundHandleRejectsWrongIdentityAndStopsChild(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestProcessHelper$")
	cmd.Env = append(os.Environ(), "PORTGUARD_TEST_CHILD=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if _, err = strconv.Atoi(line[:len(line)-1]); err != nil {
		t.Fatal(err)
	}
	p, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	created, err := p.CreateTime()
	if err != nil {
		t.Fatal(err)
	}
	info := inspect.Process{PID: int32(cmd.Process.Pid), StartedAt: created + 1}
	if h, err := openTarget(info, true); err == nil {
		h.Close()
		t.Fatal("accepted wrong process creation time")
	}
	info.StartedAt = created
	h, err := openTarget(info, true)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err = h.Signal(true); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		alive, err := h.Alive()
		if err != nil {
			t.Fatal(err)
		}
		if !alive {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("test-owned child did not exit")
}
