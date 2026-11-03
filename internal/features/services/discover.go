// Package services reads service-manager evidence; it never infers services from port numbers.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Hell077/portguard/internal/platform/command"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func Discover(ctx context.Context, pids []int32) (map[int32]string, []string) {
	result := make(map[int32]string)
	switch runtime.GOOS {
	case "linux":
		for _, pid := range pids {
			data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
			if err == nil {
				if name := FromCgroup(string(data)); name != "" {
					result[pid] = name
				}
			}
		}
	case "darwin":
		data, err := command.Output(ctx, "launchctl", "list")
		if err != nil {
			return result, []string{"launchd context unavailable"}
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 3 {
				continue
			}
			pid, err := strconv.ParseInt(fields[0], 10, 32)
			if err == nil && pid > 0 {
				result[int32(pid)] = fields[2]
			}
		}
	case "windows":
		data, err := command.Output(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `@(Get-CimInstance Win32_Service | Where-Object ProcessId -gt 0 | Select-Object ProcessId,Name) | ConvertTo-Json -Compress`)
		if err != nil {
			return result, []string{"Windows service context unavailable"}
		}
		var services []struct {
			ProcessID int32
			Name      string
		}
		if err := json.Unmarshal(data, &services); err != nil {
			return result, []string{"Windows service response could not be decoded"}
		}
		for _, s := range services {
			if result[s.ProcessID] != "" {
				result[s.ProcessID] += ", "
			}
			result[s.ProcessID] += s.Name
		}
	}
	return result, nil
}

func FromCgroup(data string) string {
	for line := range strings.SplitSeq(data, "\n") {
		for _, part := range strings.Split(line, "/") {
			if strings.HasSuffix(part, ".service") && !strings.HasPrefix(filepath.Base(part), "user@") {
				return part
			}
		}
	}
	return ""
}
