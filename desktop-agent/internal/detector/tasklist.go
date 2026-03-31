package detector

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

//list the currently running executables on the system. this is used to check if the user has a browser open, and if so, which one, so we can open the problem in that browser. this is windows specific for now, but we can add support for other platforms later if needed.
func ListRunningExecutables() (map[string]struct{}, error) {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	configureBackgroundProcess(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run tasklist: %w", err)
	}

	result := make(map[string]struct{})
	r := csv.NewReader(bytes.NewReader(out))

	for {
		record, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parse tasklist csv: %w", err)
		}
		if len(record) == 0 {
			continue
		}
		exe := strings.ToLower(strings.TrimSpace(record[0]))
		if exe == "" {
			continue
		}
		result[exe] = struct{}{}
	}
	return result, nil
}
