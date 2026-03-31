package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type rawConfig struct {
	PollIntervalSeconds int      `json:"poll_interval_seconds"`
	CooldownSeconds     int      `json:"cooldown_seconds"`
	LeetCodeProblemURL  string   `json:"leetcode_problem_url"`
	WatchedExecutables  []string `json:"watched_executables"`
	LogFilePath         string   `json:"log_file_path"`
	DryRun              bool     `json:"dry_run"`
	LogPolls            bool     `json:"log_polls"`
}

type Config struct {
	PollInterval       time.Duration
	Cooldown           time.Duration
	LeetCodeProblemURL string
	WatchedExecutables []string
	LogFilePath        string
	DryRun             bool
	LogPolls           bool
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var raw rawConfig
	if err := json.Unmarshal(b, &raw); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}

	if raw.PollIntervalSeconds <= 0 {
		raw.PollIntervalSeconds = 5
	}
	if raw.CooldownSeconds < 0 {
		raw.CooldownSeconds = 900
	}
	if strings.TrimSpace(raw.LeetCodeProblemURL) == "" {
		return Config{}, fmt.Errorf("leetcode_problem_url is required")
	}
	if len(raw.WatchedExecutables) == 0 {
		return Config{}, fmt.Errorf("watched_executables must contain at least one executable name")
	}
	if strings.TrimSpace(raw.LogFilePath) == "" {
		raw.LogFilePath = "logs/enforcement.jsonl"
	}

	watched := make([]string, 0, len(raw.WatchedExecutables))
	for _, exe := range raw.WatchedExecutables {
		trimmed := strings.ToLower(strings.TrimSpace(exe))
		if trimmed == "" {
			continue
		}
		watched = append(watched, trimmed)
	}
	if len(watched) == 0 {
		return Config{}, fmt.Errorf("watched_executables did not contain valid executable names")
	}

	return Config{
		PollInterval:       time.Duration(raw.PollIntervalSeconds) * time.Second,
		Cooldown:           time.Duration(raw.CooldownSeconds) * time.Second,
		LeetCodeProblemURL: strings.TrimSpace(raw.LeetCodeProblemURL),
		WatchedExecutables: watched,
		LogFilePath:        strings.TrimSpace(raw.LogFilePath),
		DryRun:             raw.DryRun,
		LogPolls:           raw.LogPolls,
	}, nil
}
