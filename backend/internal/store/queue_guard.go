package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// EnsureTodayQueueMatchesCategory verifies that today's assigned queue matches
// the user's selected concept tags. If a mismatch is detected, it force-resets
// today's queue and regenerates a fresh 3-problem batch from the selected tags.
func (db *DB) EnsureTodayQueueMatchesCategory(ctx context.Context, userID string) error {
	conceptCodes, err := db.userConceptCodes(ctx, userID)
	if err != nil {
		return err
	}
	tagFilters := tagsForConceptCodes(conceptCodes)
	if len(tagFilters) == 0 {
		return nil
	}

	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userTZ := "America/New_York"
	if err := tx.QueryRow(ctx, `SELECT timezone FROM users WHERE id = $1`, userID).Scan(&userTZ); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("load user timezone: %w", err)
	}
	loc, err := time.LoadLocation(userTZ)
	if err != nil {
		loc = time.UTC
	}
	today := startOfDayInLocation(time.Now(), loc)

	var mismatchedAssignedCount int
	if err := tx.QueryRow(ctx, `
        SELECT COUNT(1)
        FROM daily_assignments da
        JOIN problems p ON p.id = da.problem_id
        WHERE da.user_id = $1
          AND da.assignment_date = $2
          AND da.status = 'ASSIGNED'
          AND NOT EXISTS (
              SELECT 1
              FROM unnest($3::text[]) AS preferred_tag
              WHERE preferred_tag = ANY(p.tags)
          )
    `, userID, today, tagFilters).Scan(&mismatchedAssignedCount); err != nil {
		return fmt.Errorf("check queue/category mismatch: %w", err)
	}
	if mismatchedAssignedCount == 0 {
		return nil
	}

	if _, err := tx.Exec(ctx, `
        DELETE FROM daily_assignments
        WHERE user_id = $1
          AND assignment_date = $2
    `, userID, today); err != nil {
		return fmt.Errorf("clear mismatched queue: %w", err)
	}

	candidates, err := queryQueueCandidates(ctx, tx, userID, dailyLimit, tagFilters)
	if err != nil {
		return err
	}
	for i, c := range candidates {
		position := int16(i + 1)
		if _, err := tx.Exec(ctx, `
            INSERT INTO daily_assignments (user_id, problem_id, assignment_date, position, status)
            VALUES ($1, $2, $3, $4, 'ASSIGNED')
            ON CONFLICT (user_id, assignment_date, position) DO NOTHING
        `, userID, c.ProblemID, today, position); err != nil {
			return fmt.Errorf("insert guarded queue assignment: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit queue/category guard tx: %w", err)
	}
	return nil
}
