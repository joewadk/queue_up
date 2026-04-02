//go:build windows

package tray

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const queueUpAgentExecutable = "queue-up-agent.exe"
const noTasksInfo = "INFO: No tasks are running which match the specified criteria."

func terminateQueueUpAgents() {
	pids, err := queueUpAgentPIDs()
	if err != nil {
		log.Printf("queue-up-agent lookup failed: %v", err)
		return
	}

	if len(pids) == 0 {
		return
	}

	selfPID := os.Getpid()
	killed := 0
	for _, pid := range pids {
		if pid == selfPID {
			continue
		}
		if err := killQueueUpAgent(pid); err != nil {
			log.Printf("failed to terminate queue-up-agent pid=%d: %v", pid, err)
			continue
		}
		killed++
	}
	if killed > 0 {
		log.Printf("terminated %d other queue-up-agent process(es)", killed)
	}
}

func queueUpAgentPIDs() ([]int, error) {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+queueUpAgentExecutable, "/FO", "CSV", "/NH")
	hideCommandWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run tasklist: %w", err)
	}

	payload := strings.TrimSpace(string(out))
	if payload == "" {
		return nil, nil
	}
	if strings.HasPrefix(payload, "INFO:") {
		if strings.Contains(payload, noTasksInfo) {
			return nil, nil
		}
		return nil, fmt.Errorf("tasklist reported info: %s", payload)
	}

	reader := csv.NewReader(strings.NewReader(payload))
	pids := make([]int, 0, 2)
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parse tasklist csv: %w", err)
		}
		if len(record) < 2 {
			continue
		}
		pidStr := strings.TrimSpace(record[1])
		if pidStr == "" {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func killQueueUpAgent(pid int) error {
	cmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	hideCommandWindow(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("taskkill: %w", err)
	}
	return nil
}

func hideCommandWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
