package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var errSubmissionSanitizerWebhookDisabled = errors.New("submission sanitizer webhook is not configured")

type CompletionInput struct {
	UserID        string
	ProblemID     int64
	CompletedAt   time.Time
	Source        string
	Verification  string
	SubmissionURL string
}

func (db *DB) RecordCompletion(ctx context.Context, in CompletionInput) error {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userTZ := "America/New_York"
	if err := tx.QueryRow(ctx, `SELECT timezone FROM users WHERE id = $1`, in.UserID).Scan(&userTZ); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("load user: %w", err)
	}

	var problemSlug string
	if err := tx.QueryRow(ctx, `SELECT slug FROM problems WHERE id = $1`, in.ProblemID).Scan(&problemSlug); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("problem not found")
		}
		return fmt.Errorf("load problem slug: %w", err)
	}
	submissionURL, err := db.sanitizeSubmissionURLViaWebhook(ctx, in.UserID, in.ProblemID, problemSlug, in.SubmissionURL)
	if err != nil {
		if errors.Is(err, errSubmissionSanitizerWebhookDisabled) {
			return fmt.Errorf("submission sanitizer webhook is required but not configured")
		}
		return err
	}
	if strings.TrimSpace(problemSlug) == "" {
		return fmt.Errorf("problem not found")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO problem_completions (user_id, problem_id, completed_at, source, verification, submission_url)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
	`, in.UserID, in.ProblemID, in.CompletedAt, in.Source, in.Verification, submissionURL); err != nil {
		return fmt.Errorf("insert completion: %w", err)
	}

	loc, err := time.LoadLocation(userTZ)
	if err != nil {
		loc = time.UTC
	}
	assignmentDate := startOfDayInLocation(in.CompletedAt, loc)
	if _, err := tx.Exec(ctx, `
		UPDATE daily_assignments
		SET status = 'COMPLETED'
		WHERE user_id = $1
		  AND problem_id = $2
		  AND assignment_date = $3
	`, in.UserID, in.ProblemID, assignmentDate); err != nil {
		return fmt.Errorf("update assignment status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func sanitizeSubmissionURL(raw, expectedSlug string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid submission_url")
	}
	host := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(u.Hostname()), "www."))
	if host != "leetcode.com" {
		return "", fmt.Errorf("submission_url must be from leetcode.com")
	}

	pathParts := strings.Split(strings.Trim(strings.TrimSpace(u.Path), "/"), "/")
	if len(pathParts) < 4 || pathParts[0] != "problems" || pathParts[2] != "submissions" {
		return "", fmt.Errorf("submission_url must match /problems/{slug}/submissions/{id}")
	}

	slug := strings.TrimSpace(pathParts[1])
	if slug == "" {
		return "", fmt.Errorf("submission_url is missing problem slug")
	}
	if strings.TrimSpace(expectedSlug) != "" && !strings.EqualFold(slug, strings.TrimSpace(expectedSlug)) {
		return "", fmt.Errorf("submission_url problem does not match selected queue problem")
	}

	submissionID, parseErr := strconv.ParseInt(strings.TrimSpace(pathParts[3]), 10, 64)
	if parseErr != nil || submissionID <= 0 {
		return "", fmt.Errorf("submission_url has invalid submission id")
	}

	return fmt.Sprintf("https://leetcode.com/problems/%s/submissions/%d/", slug, submissionID), nil
}
