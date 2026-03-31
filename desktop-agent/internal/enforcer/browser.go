package enforcer

import (
	"fmt"
	"os/exec"
	"strings"
)

func OpenInDefaultBrowser(url string) error {
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		return fmt.Errorf("empty url")
	}

	//windows uses rundll32 to open the default browser with the given URL
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", trimmed)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start browser command: %w", err)
	}
	return nil
}
