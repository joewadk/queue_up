package eventlogger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type EnforcementEvent struct {
	TimestampUTC          time.Time `json:"timestamp_utc"`
	Executable            string    `json:"executable"`
	Action                string    `json:"action"`
	ProblemURL            string    `json:"problem_url"`
	RecommendationSource  string    `json:"recommendation_source,omitempty"`
	RecommendedProblemSlug string   `json:"recommended_problem_slug,omitempty"`
	DryRun                bool      `json:"dry_run"`
	Error                 string    `json:"error,omitempty"`
}

type JSONL struct {
	mu sync.Mutex
	f  *os.File
}

func NewJSONL(path string) (*JSONL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &JSONL{f: f}, nil
}

func (l *JSONL) Write(event EnforcementEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if _, err := l.f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	return nil
}

func (l *JSONL) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	return l.f.Close()
}
