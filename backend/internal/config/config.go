package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr                          string
	DatabaseURL                       string
	SubmissionSanitizerWebhookURL     string
	SubmissionSanitizerWebhookTimeout time.Duration
}

// load in the env vars
func Load() Config {
	return Config{
		HTTPAddr:                          envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:                       envOrDefault("DATABASE_URL", "postgres://queue_up@localhost:5432/queue_up?sslmode=disable"),
		SubmissionSanitizerWebhookURL:     envOrDefault("SUBMISSION_SANITIZER_WEBHOOK_URL", ""),
		SubmissionSanitizerWebhookTimeout: envDurationOrDefault("SUBMISSION_SANITIZER_WEBHOOK_TIMEOUT_MS", 3000*time.Millisecond),
	}
}

// helper function to read env vars with a default fallback
func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	ms, err := strconv.Atoi(v)
	if err != nil || ms <= 0 {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}
