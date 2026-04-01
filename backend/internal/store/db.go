package store

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool

	submissionSanitizerWebhookURL string
	submissionSanitizerHTTPClient *http.Client
	leetCodeAPIBaseURL            string
	leetCodeAPIHTTPClient         *http.Client
}

// postgres connection pool setup and helper methods for interacting with the db
func Open(databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) ConfigureSubmissionSanitizerWebhook(url string, timeout time.Duration) {
	db.submissionSanitizerWebhookURL = strings.TrimSpace(url)
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	db.submissionSanitizerHTTPClient = &http.Client{Timeout: timeout}
}

func (db *DB) ConfigureLeetCodeAPI(baseURL string, timeout time.Duration) {
	db.leetCodeAPIBaseURL = strings.TrimSpace(baseURL)
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	db.leetCodeAPIHTTPClient = &http.Client{Timeout: timeout}
}
