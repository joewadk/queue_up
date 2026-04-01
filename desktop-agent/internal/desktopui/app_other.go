//go:build !windows

package desktopui

import (
	"fmt"

	"queue_up/desktop-agent/internal/config"
)

func Run(cfg config.Config) error {
	return fmt.Errorf("native desktop ui is currently supported on Windows only")
}
