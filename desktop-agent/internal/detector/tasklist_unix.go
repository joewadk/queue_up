//go:build !windows

package detector

import (
	"fmt"
	"os/exec"
	"strings"
)

// ListRunningExecutables is used on non-Windows platforms to collect running binary names.
func ListRunningExecutables() (map[string]struct{}, error) {
	cmd := exec.Command("ps", "-eo", "comm=")
	configureBackgroundProcess(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run ps: %w", err)
	}

	result := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		exe := strings.ToLower(strings.TrimSpace(line))
		if exe == "" {
			continue
		}
		result[exe] = struct{}{}
	}
	return result, nil
}
