//go:build !windows

package startup

import "fmt"

func Install(entryName, exePath, configPath string) error {
	return fmt.Errorf("startup registration is only supported on Windows")
}

func Uninstall(entryName string) error {
	return fmt.Errorf("startup registration is only supported on Windows")
}

func Status(entryName string) (bool, string, error) {
	return false, "", fmt.Errorf("startup registration is only supported on Windows")
}
