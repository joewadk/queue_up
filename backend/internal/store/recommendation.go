package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const dailyLimit = 3

var ErrUserNotFound = errors.New("user not found")

var conceptTagMap = map[string][]string{
	"ARRAY":               {"ARRAY_HASHING"},
	"TWO_POINTERS":        {"TWO_POINTERS"},
	"STACK":               {"STACK"},
	"BINARY_SEARCH":       {"BINARY_SEARCH"},
	"SLIDING_WINDOW":      {"SLIDING_WINDOW"},
	"LINKED_LIST":         {"LINKED_LIST"},
	"TREE":                {"TREE"},
	"TRIE":                {"TRIE"},
	"BACKTRACKING":        {"BACKTRACKING"},
	"HEAP":                {"HEAP"},
	"GRAPH":               {"GRAPH"},
	"DP_1D":               {"DP_1D"},
	"DP_2D":               {"DP_2D"},
	"INTERVALS":           {"INTERVALS"},
	"GREEDY":              {"GREEDY"},
	"BIT_MANIPULATION":    {"BIT_MANIPULATION"},
	"DSU":                 {"DSU (Disjoint Set Union)"},
	"QUEUE":               {"QUEUE"},
	"MATH_GEOMETRY":       {"MATH&GEOMETRY"},
}

func tagsForConceptCodes(codes []string) []string {
	r := make(map[string]struct{}, len(codes))
	for _, raw := range codes {
		code := strings.TrimSpace(strings.ToUpper(raw))
		if code == "" {
			continue
		}
		tags := conceptTagMap[code]
		if len(tags) == 0 {
			tags = []string{code}
		}
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			r[tag] = struct{}{}
		}
	}
	if len(r) == 0 {
		return nil
	}
	out := make([]string, 0, len(r))
	for tag := range r {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

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
	conceptCodes, err := db.userConceptCodes(ctx, userID)
	if err != nil {
		return time.Time{}, nil, err
	}
	tagFilters := tagsForConceptCodes(conceptCodes)

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

	candidates, err := queryQueueCandidates(ctx, tx, userID, dailyLimit, tagFilters)
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

func queryQueueCandidates(ctx context.Context, tx pgx.Tx, userID string, limit int, tagFilters []string) ([]Recommendation, error) {
	if limit <= 0 {
		return nil, nil
	}
	var tagArg interface{}
	if len(tagFilters) > 0 {
		tagArg = tagFilters
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
          AND (
            $2::text[] IS NULL
            OR EXISTS (
              SELECT 1
              FROM unnest($2::text[]) AS preferred_tag
              WHERE preferred_tag = ANY(p.tags)
            )
          )
        ORDER BY
            COALESCE(p.queue_rank, 1000),
            CASE p.difficulty
                WHEN 'Easy' THEN 1
                WHEN 'Medium' THEN 2
                ELSE 3
            END,
            p.id
        LIMIT $3
    `, userID, tagArg, limit)
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
