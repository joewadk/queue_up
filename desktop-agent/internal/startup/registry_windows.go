package startup

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)
//
const runKeyPath = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func Install(entryName, exePath, configPath string) error {
	entryName = strings.TrimSpace(entryName)
	if entryName == "" {
		return fmt.Errorf("startup entry name is required")
	}
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return fmt.Errorf("exe path is required")
	}
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return fmt.Errorf("config path is required")
	}

	command := fmt.Sprintf("\"%s\" -config \"%s\"", exePath, configPath)
	out, err := exec.Command("reg", "add", runKeyPath, "/v", entryName, "/t", "REG_SZ", "/d", command, "/f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("add startup registry value: %w | output=%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Uninstall(entryName string) error {
	entryName = strings.TrimSpace(entryName)
	if entryName == "" {
		return fmt.Errorf("startup entry name is required")
	}

	out, err := exec.Command("reg", "delete", runKeyPath, "/v", entryName, "/f").CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out)), "unable to find") {
			return nil
		}
		return fmt.Errorf("delete startup registry value: %w | output=%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Status(entryName string) (enabled bool, command string, err error) {
	entryName = strings.TrimSpace(entryName)
	if entryName == "" {
		return false, "", fmt.Errorf("startup entry name is required")
	}

	out, cmdErr := exec.Command("reg", "query", runKeyPath, "/v", entryName).CombinedOutput()
	if cmdErr != nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "unable to find") || strings.Contains(lower, "the system was unable") {
			return false, "", nil
		}
		return false, "", fmt.Errorf("query startup registry value: %w | output=%s", cmdErr, strings.TrimSpace(string(out)))
	}

	line, parseErr := extractRegSzValue(string(out))
	if parseErr != nil {
		return false, "", parseErr
	}
	return true, line, nil
}

func extractRegSzValue(output string) (string, error) {
	lines := strings.Split(output, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		fieldLower := strings.ToUpper(fields[1])
		if !strings.Contains(fieldLower, "REG_SZ") {
			continue
		}
		idx := strings.Index(line, fields[1])
		if idx < 0 {
			continue
		}
		value := strings.TrimSpace(line[idx+len(fields[1]):])
		if value == "" {
			return "", errors.New("startup registry value is empty")
		}
		return value, nil
	}
	return "", errors.New("unable to parse startup registry value")
}
