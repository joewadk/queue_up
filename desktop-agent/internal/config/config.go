package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type rawConfig struct {
	PollIntervalSeconds int      `json:"poll_interval_seconds"`
	CooldownSeconds     int      `json:"cooldown_seconds"`
	LeetCodeProblemURL  string   `json:"leetcode_problem_url"`
	BackendBaseURL      string   `json:"backend_base_url"`
	UserID              string   `json:"user_id"`
	RequestTimeoutSec   int      `json:"request_timeout_seconds"`
	WatchedExecutables  []string `json:"watched_executables"`
	LogFilePath         string   `json:"log_file_path"`
	DryRun              bool     `json:"dry_run"`
	LogPolls            bool     `json:"log_polls"`
	EnableTray          bool     `json:"enable_tray"`
	OpenGUIOnStart      bool     `json:"open_gui_on_start"`
}

type Config struct {
	PollInterval       time.Duration
	Cooldown           time.Duration
	LeetCodeProblemURL string
	BackendBaseURL     string
	UserID             string
	RequestTimeout     time.Duration
	WatchedExecutables []string
	LogFilePath        string
	DryRun             bool
	LogPolls           bool
	EnableTray         bool
	OpenGUIOnStart     bool
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
	if raw.RequestTimeoutSec <= 0 {
		raw.RequestTimeoutSec = 5
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

	configDir := filepath.Dir(path)
	logPath := strings.TrimSpace(raw.LogFilePath)
	if !filepath.IsAbs(logPath) {
		logPath = filepath.Join(configDir, logPath)
	}

	return Config{
		PollInterval:       time.Duration(raw.PollIntervalSeconds) * time.Second,
		Cooldown:           time.Duration(raw.CooldownSeconds) * time.Second,
		LeetCodeProblemURL: strings.TrimSpace(raw.LeetCodeProblemURL),
		BackendBaseURL:     strings.TrimSpace(raw.BackendBaseURL),
		UserID:             strings.TrimSpace(raw.UserID),
		RequestTimeout:     time.Duration(raw.RequestTimeoutSec) * time.Second,
		WatchedExecutables: watched,
		LogFilePath:        logPath,
		DryRun:             raw.DryRun,
		LogPolls:           raw.LogPolls,
		EnableTray:         raw.EnableTray,
		OpenGUIOnStart:     raw.OpenGUIOnStart,
	}, nil
}

func UpdateUserID(path, userID string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("config path is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if existing, ok := doc["user_id"].(string); ok && strings.TrimSpace(existing) == userID {
		return nil
	}

	doc["user_id"] = userID
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	encoded = append(encoded, '\n')

	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "queue-up-config-*.json")
	if err != nil {
		return fmt.Errorf("create temp config in %s: %w", dir, err)
	}
	tempPath := temp.Name()
	if _, err := temp.Write(encoded); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
