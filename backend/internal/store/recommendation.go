package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const dailyLimit = 3

var ErrUserNotFound = errors.New("user not found")

type Recommendation struct {
	Position   int16  `json:"position"`
	ProblemID  int64  `json:"problem_id"`
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Difficulty string `json:"difficulty"`
}

// get or create today's recommendations for a user. if the user already has assignments for today, return those.
func (db *DB) GetOrCreateTodayRecommendations(ctx context.Context, userID string) (time.Time, []Recommendation, error) {
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
	today := startOfDayInLocation(time.Now(), loc)

	existing, err := queryAssignments(ctx, tx, userID, today)
	if err != nil {
		return time.Time{}, nil, err
	}
	if len(existing) > 0 {
		if err := tx.Commit(ctx); err != nil {
			return time.Time{}, nil, fmt.Errorf("commit tx existing: %w", err)
		}
		return today, existing, nil
	}

	candidates, err := queryQueueCandidates(ctx, tx, userID, dailyLimit)
	if err != nil {
		return time.Time{}, nil, err
	}

	for i, c := range candidates {
		position := int16(i + 1)
		if _, err := tx.Exec(ctx, `
            INSERT INTO daily_assignments (user_id, problem_id, assignment_date, position, status)
            VALUES ($1, $2, $3, $4, 'ASSIGNED')
            ON CONFLICT (user_id, assignment_date, position) DO NOTHING
        `, userID, c.ProblemID, today, position); err != nil {
			return time.Time{}, nil, fmt.Errorf("insert assignment problem_id=%d: %w", c.ProblemID, err)
		}
	}

	assignments, err := queryAssignments(ctx, tx, userID, today)
	if err != nil {
		return time.Time{}, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, nil, fmt.Errorf("commit tx new assignments: %w", err)
	}
	return today, assignments, nil
}

func queryAssignments(ctx context.Context, tx pgx.Tx, userID string, date time.Time) ([]Recommendation, error) {
	rows, err := tx.Query(ctx, `
        SELECT
            da.position,
            p.id,
            p.slug,
            p.title,
            p.url,
            p.difficulty
        FROM daily_assignments da
        JOIN problems p ON p.id = da.problem_id
        WHERE da.user_id = $1
          AND da.assignment_date = $2
          AND da.status = 'ASSIGNED'
        ORDER BY da.position
    `, userID, date)
	if err != nil {
		return nil, fmt.Errorf("query existing assignments: %w", err)
	}
	defer rows.Close()

	var out []Recommendation
	for rows.Next() {
		var r Recommendation
		if err := rows.Scan(
			&r.Position,
			&r.ProblemID,
			&r.Slug,
			&r.Title,
			&r.URL,
			&r.Difficulty,
		); err != nil {
			return nil, fmt.Errorf("scan assignment row: %w", err)
		}
		out = append(out, r)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate assignments: %w", rows.Err())
	}
	return out, nil
}

func queryQueueCandidates(ctx context.Context, tx pgx.Tx, userID string, limit int) ([]Recommendation, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, `
        SELECT
            0::smallint AS position,
            p.id,
            p.slug,
            p.title,
            p.url,
            p.difficulty
        FROM problems p
        WHERE p.active = true
          AND p.source_set IN ('NEETCODE_150', 'LEETCODE_API')
          AND NOT EXISTS (
            SELECT 1
            FROM problem_completions pc
            WHERE pc.user_id = $1
              AND pc.problem_id = p.id
          )
        ORDER BY
            COALESCE(p.queue_rank, 1000),
            CASE p.difficulty
                WHEN 'Easy' THEN 1
                WHEN 'Medium' THEN 2
                ELSE 3
            END,
            p.id
        LIMIT $2
    `, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query queue candidates: %w", err)
	}
	defer rows.Close()

	var out []Recommendation
	for rows.Next() {
		var r Recommendation
		if err := rows.Scan(
			&r.Position,
			&r.ProblemID,
			&r.Slug,
			&r.Title,
			&r.URL,
			&r.Difficulty,
		); err != nil {
			return nil, fmt.Errorf("scan queue candidate: %w", err)
		}
		out = append(out, r)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate queue candidates: %w", rows.Err())
	}
	return out, nil
}
