//go:build windows && no_gl

package desktopui

import (
	"fmt"

	"queue_up/desktop-agent/internal/config"
)

func Run(cfg config.Config, configPath string) error {
	_ = cfg
	_ = configPath
	return fmt.Errorf("native desktop ui is disabled in no_gl builds; rebuild without -tags no_gl and with CGO_ENABLED=1 plus gcc in PATH")
}
