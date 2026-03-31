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
	Position    int16  `json:"position"`
	ProblemID   int64  `json:"problem_id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Difficulty  string `json:"difficulty"`
	ConceptCode string `json:"concept_code"`
	ConceptName string `json:"concept_name"`
}
//get or create today's recommendations for a user. if the user already has assignments for today, return those.
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

	prefIDs, err := queryPreferredConceptIDs(ctx, tx, userID)
	if err != nil {
		return time.Time{}, nil, err
	}

	candidates, err := queryCandidates(ctx, tx, userID, prefIDs)
	if err != nil {
		return time.Time{}, nil, err
	}

	for i, c := range candidates {
		position := int16(i + 1)
		_, err := tx.Exec(ctx, `
			INSERT INTO daily_assignments (user_id, problem_id, assignment_date, position, status)
			VALUES ($1, $2, $3, $4, 'ASSIGNED')
			ON CONFLICT (user_id, assignment_date, position) DO NOTHING
		`, userID, c.ProblemID, today, position)
		if err != nil {
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
//helper functions for querying the db for recommendations and related data. 
//these are used in the GetOrCreateTodayRecommendations method above.
func queryAssignments(ctx context.Context, tx pgx.Tx, userID string, date time.Time) ([]Recommendation, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			da.position,
			p.id,
			p.slug,
			p.title,
			p.url,
			p.difficulty,
			COALESCE(c.code, ''),
			COALESCE(c.display_name, '')
		FROM daily_assignments da
		JOIN problems p ON p.id = da.problem_id
		LEFT JOIN LATERAL (
			SELECT c.code, c.display_name
			FROM problem_concepts pc
			JOIN concepts c ON c.id = pc.concept_id
			WHERE pc.problem_id = p.id
			ORDER BY c.id
			LIMIT 1
		) c ON true
		WHERE da.user_id = $1
		  AND da.assignment_date = $2
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
			&r.ConceptCode,
			&r.ConceptName,
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
//helper functions for querying the db for recommendations and related data. 
//these are used in the GetOrCreateTodayRecommendations method above.
func queryPreferredConceptIDs(ctx context.Context, tx pgx.Tx, userID string) ([]int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT concept_id
		FROM user_concept_preferences
		WHERE user_id = $1
		ORDER BY selected_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query concept preferences: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan concept preference: %w", err)
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate concept preferences: %w", rows.Err())
	}
	return ids, nil
}

func queryCandidates(ctx context.Context, tx pgx.Tx, userID string, prefIDs []int64) ([]Recommendation, error) {
	base := `
		WITH ranked AS (
			SELECT
				p.id,
				p.slug,
				p.title,
				p.url,
				p.difficulty,
				c.code,
				c.display_name,
				ROW_NUMBER() OVER (PARTITION BY p.id ORDER BY c.id) AS rn
			FROM problems p
			JOIN problem_concepts pc ON pc.problem_id = p.id
			JOIN concepts c ON c.id = pc.concept_id
			WHERE p.active = true
			  AND p.source_set = 'NEETCODE_150'
			  AND p.difficulty = 'Easy'
			  AND NOT EXISTS (
				SELECT 1
				FROM problem_attempts pa
				WHERE pa.user_id = $1
				  AND pa.problem_id = p.id
			  )
			  AND NOT EXISTS (
				SELECT 1
				FROM daily_assignments da
				WHERE da.user_id = $1
				  AND da.problem_id = p.id
			  )
	`
	var rows pgx.Rows
	var err error
	if len(prefIDs) > 0 {
		rows, err = tx.Query(ctx, base+`
			  AND pc.concept_id = ANY($2)
		)
		SELECT
			0::smallint AS position,
			id,
			slug,
			title,
			url,
			difficulty,
			code,
			display_name
		FROM ranked
		WHERE rn = 1
		ORDER BY id
		LIMIT 3
	`, userID, prefIDs)
	} else {
		rows, err = tx.Query(ctx, base+`
		)
		SELECT
			0::smallint AS position,
			id,
			slug,
			title,
			url,
			difficulty,
			code,
			display_name
		FROM ranked
		WHERE rn = 1
		ORDER BY id
		LIMIT 3
	`, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("query candidates: %w", err)
	}
	defer rows.Close()

	out := make([]Recommendation, 0, dailyLimit)
	for rows.Next() {
		var r Recommendation
		if err := rows.Scan(
			&r.Position,
			&r.ProblemID,
			&r.Slug,
			&r.Title,
			&r.URL,
			&r.Difficulty,
			&r.ConceptCode,
			&r.ConceptName,
		); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		out = append(out, r)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate candidates: %w", rows.Err())
	}
	return out, nil
}
