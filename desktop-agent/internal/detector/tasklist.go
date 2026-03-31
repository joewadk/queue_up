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

// ListRunningExecutables returns a case-insensitive set of running exe names.
func ListRunningExecutables() (map[string]struct{}, error) {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
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
