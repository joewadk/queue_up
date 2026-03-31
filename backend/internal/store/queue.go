package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type QueueItem struct {
	Position    int16  `json:"position"`
	ProblemID   int64  `json:"problem_id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Difficulty  string `json:"difficulty"`
	IsCompleted bool   `json:"is_completed"`
}
//get the daily queue for a user on a specific date, defaulting to today if no date is provided. 
//handles timezone conversion based on the user's settings. 
//returns the queue along with the date it corresponds to (in case of timezone adjustments)
func (db *DB) GetDailyQueue(ctx context.Context, userID string, date *time.Time) (time.Time, []QueueItem, error) {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userTZ := "America/New_York"
	if err := tx.QueryRow(ctx, `SELECT timezone FROM users WHERE id = $1`, userID).Scan(&userTZ); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, nil, ErrUserNotFound
		}
		return time.Time{}, nil, fmt.Errorf("load user: %w", err)
	}

	loc, err := time.LoadLocation(userTZ)
	if err != nil {
		loc = time.UTC
	}

	targetDate := startOfDayInLocation(time.Now(), loc)
	if date != nil {
		targetDate = startOfDayInLocation(*date, loc)
	}

	rows, err := tx.Query(ctx, `
		SELECT
			da.position,
			p.id,
			p.slug,
			p.title,
			p.url,
			p.difficulty,
			(da.status = 'COMPLETED') AS is_completed
		FROM daily_assignments da
		JOIN problems p ON p.id = da.problem_id
		WHERE da.user_id = $1
		  AND da.assignment_date = $2
		ORDER BY da.position
	`, userID, targetDate)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("query daily queue: %w", err)
	}
	defer rows.Close()

	var out []QueueItem
	for rows.Next() {
		var q QueueItem
		if err := rows.Scan(
			&q.Position,
			&q.ProblemID,
			&q.Slug,
			&q.Title,
			&q.URL,
			&q.Difficulty,
			&q.IsCompleted,
		); err != nil {
			return time.Time{}, nil, fmt.Errorf("scan daily queue: %w", err)
		}
		out = append(out, q)
	}
	if rows.Err() != nil {
		return time.Time{}, nil, fmt.Errorf("iterate daily queue: %w", rows.Err())
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, nil, fmt.Errorf("commit tx: %w", err)
	}
	return targetDate, out, nil
}
