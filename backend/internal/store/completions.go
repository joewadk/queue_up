package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type CompletionInput struct {
	UserID       string
	ProblemID    int64
	CompletedAt  time.Time
	Source       string
	Verification string
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

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM problems WHERE id = $1)`, in.ProblemID).Scan(&exists); err != nil {
		return fmt.Errorf("check problem exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("problem not found")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO problem_completions (user_id, problem_id, completed_at, source, verification, submission_url)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''))
	`, in.UserID, in.ProblemID, in.CompletedAt, in.Source, in.Verification, in.SubmissionURL); err != nil {
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

	if _, err := tx.Exec(ctx, `
		INSERT INTO problem_attempts (user_id, problem_id, assignment_date, quality, attempted_at)
		VALUES ($1, $2, $3, 5, $4)
	`, in.UserID, in.ProblemID, assignmentDate, in.CompletedAt); err != nil {
		return fmt.Errorf("insert attempt snapshot: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
