package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"queue_up/desktop-agent/internal/config"
	"queue_up/desktop-agent/internal/detector"
	"queue_up/desktop-agent/internal/enforcer"
	"queue_up/desktop-agent/internal/eventlogger"
)

func main() {
	configPath := flag.String("config", "config/config.json", "Path to desktop agent config JSON")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger, err := eventlogger.NewJSONL(cfg.LogFilePath)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer logger.Close()

	fmt.Printf("queue-up-agent started | poll=%s cooldown=%s watched=%v\n",
		cfg.PollInterval, cfg.Cooldown, cfg.WatchedExecutables)

	lastTriggered := make(map[string]time.Time)
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	// Run one check immediately on startup.
	runOnce(cfg, logger, lastTriggered)

	for range ticker.C {
		runOnce(cfg, logger, lastTriggered)
	}
}

func runOnce(cfg config.Config, logger *eventlogger.JSONL, lastTriggered map[string]time.Time) {
	processes, err := detector.ListRunningExecutables()
	if err != nil {
		log.Printf("list processes: %v", err)
		return
	}

	now := time.Now().UTC()
	matched := make([]string, 0, len(cfg.WatchedExecutables))
	for _, exe := range cfg.WatchedExecutables {
		if _, found := processes[exe]; !found {
			continue
		}
		matched = append(matched, exe)

		last, hasLast := lastTriggered[exe]
		if hasLast && now.Sub(last) < cfg.Cooldown {
			continue
		}

		event := eventlogger.EnforcementEvent{
			TimestampUTC: now,
			Executable:   exe,
			Action:       "OPEN_LEETCODE_PROBLEM",
			ProblemURL:   cfg.LeetCodeProblemURL,
			DryRun:       cfg.DryRun,
		}

		if !cfg.DryRun {
			if err := enforcer.OpenInDefaultBrowser(cfg.LeetCodeProblemURL); err != nil {
				event.Action = "OPEN_LEETCODE_PROBLEM_FAILED"
				event.Error = err.Error()
				log.Printf("open browser for %s: %v", exe, err)
			}
		}

		if err := logger.Write(event); err != nil {
			log.Printf("write event log: %v", err)
		}

		lastTriggered[exe] = now
		if cfg.DryRun {
			log.Printf("detected %s (dry-run): would open %s", exe, cfg.LeetCodeProblemURL)
		} else {
			log.Printf("detected %s: opened %s", exe, cfg.LeetCodeProblemURL)
		}
	}

	if cfg.LogPolls {
		if len(matched) == 0 {
			log.Printf("poll tick: scanned running processes, no watched executables found")
		} else {
			log.Printf("poll tick: watched executables currently running: %v", matched)
		}
	}
}

func init() {
	// Keep Windows console window alive on fatal startup errors when double-clicked.
	if len(os.Args) == 1 {
		_ = os.Setenv("QUEUE_UP_INTERACTIVE", "1")
	}
}
