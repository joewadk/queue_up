//go:build !windows

package detector

import "os/exec"

func configureBackgroundProcess(cmd *exec.Cmd) {}
