package config

import "os"

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}
//load in the env vars 
func Load() Config {
	return Config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://queue_up@localhost:5432/queue_up?sslmode=disable"),
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
