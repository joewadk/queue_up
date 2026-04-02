package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var leetCodeUsernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

type Concept struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
}

type UserProfile struct {
	UserID             string   `json:"user_id"`
	LeetCodeUsername   string   `json:"leetcode_username"`
	Timezone           string   `json:"timezone"`
	OnboardingComplete bool     `json:"onboarding_complete"`
	ConceptCodes       []string `json:"concept_codes"`
}

type CompletedProblem struct {
	ProblemID     int64  `json:"problem_id"`
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	Difficulty    string `json:"difficulty"`
	CompletedAt   string `json:"completed_at"`
	SubmissionURL string `json:"submission_url,omitempty"`
}

func (db *DB) ListConcepts(ctx context.Context) ([]Concept, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT code, display_name
		FROM concepts
		ORDER BY display_name
	`)
	if err != nil {
		return nil, fmt.Errorf("query concepts: %w", err)
	}
	defer rows.Close()

	out := make([]Concept, 0)
	for rows.Next() {
		var c Concept
		if err := rows.Scan(&c.Code, &c.DisplayName); err != nil {
			return nil, fmt.Errorf("scan concept: %w", err)
		}
		out = append(out, c)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate concepts: %w", rows.Err())
	}
	return out, nil
}

func (db *DB) GetUserByLeetCodeUsername(ctx context.Context, username string) (UserProfile, bool, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return UserProfile{}, false, nil
	}

	var profile UserProfile
	err := db.pool.QueryRow(ctx, `
		SELECT id::text, leetcode_username, timezone, onboarding_completed
		FROM users
		WHERE LOWER(leetcode_username) = LOWER($1)
	`, username).Scan(&profile.UserID, &profile.LeetCodeUsername, &profile.Timezone, &profile.OnboardingComplete)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserProfile{}, false, nil
		}
		return UserProfile{}, false, fmt.Errorf("query user by leetcode username: %w", err)
	}

	codes, err := db.userConceptCodes(ctx, profile.UserID)
	if err != nil {
		return UserProfile{}, false, err
	}
	profile.ConceptCodes = codes
	return profile, true, nil
}

func (db *DB) BootstrapUser(ctx context.Context, username, timezone string) (UserProfile, bool, error) {
	username = strings.TrimSpace(username)
	if !leetCodeUsernamePattern.MatchString(username) {
		return UserProfile{}, false, fmt.Errorf("invalid leetcode username format")
	}
	if strings.TrimSpace(timezone) == "" {
		timezone = "America/New_York"
	}

	if _, err := time.LoadLocation(timezone); err != nil {
		return UserProfile{}, false, fmt.Errorf("invalid timezone")
	}

	existing, found, err := db.GetUserByLeetCodeUsername(ctx, username)
	if err != nil {
		return UserProfile{}, false, err
	}
	if found {
		return existing, false, nil
	}

	id, err := newUUID()
	if err != nil {
		return UserProfile{}, false, err
	}

	clerkUserID := "local:" + strings.ToLower(username)
	_, err = db.pool.Exec(ctx, `
		INSERT INTO users (id, clerk_user_id, timezone, leetcode_username, onboarding_completed)
		VALUES ($1, $2, $3, $4, false)
	`, id, clerkUserID, timezone, username)
	if err != nil {
		return UserProfile{}, false, fmt.Errorf("insert user: %w", err)
	}

	return UserProfile{
		UserID:             id,
		LeetCodeUsername:   username,
		Timezone:           timezone,
		OnboardingComplete: false,
		ConceptCodes:       []string{},
	}, true, nil
}

func (db *DB) SetUserConceptPreferences(ctx context.Context, userID string, conceptCodes []string) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
		return fmt.Errorf("check user exists: %w", err)
	}
	if !exists {
		return ErrUserNotFound
	}

	normalized := make([]string, 0, len(conceptCodes))
	seen := make(map[string]struct{}, len(conceptCodes))
	for _, raw := range conceptCodes {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_concept_preferences WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear concept prefs: %w", err)
	}

	if len(normalized) > 0 {
		rows, err := tx.Query(ctx, `
			SELECT id, code
			FROM concepts
			WHERE code = ANY($1)
		`, normalized)
		if err != nil {
			return fmt.Errorf("query concept ids: %w", err)
		}
		defer rows.Close()

		conceptIDs := make([]int64, 0, len(normalized))
		foundCodes := make(map[string]struct{}, len(normalized))
		for rows.Next() {
			var id int64
			var code string
			if err := rows.Scan(&id, &code); err != nil {
				return fmt.Errorf("scan concept row: %w", err)
			}
			conceptIDs = append(conceptIDs, id)
			foundCodes[strings.ToUpper(code)] = struct{}{}
		}
		if rows.Err() != nil {
			return fmt.Errorf("iterate concept rows: %w", rows.Err())
		}

		if len(conceptIDs) != len(normalized) {
			missing := make([]string, 0)
			for _, code := range normalized {
				if _, ok := foundCodes[code]; !ok {
					missing = append(missing, code)
				}
			}
			return fmt.Errorf("unknown concept codes: %s", strings.Join(missing, ", "))
		}

		for _, conceptID := range conceptIDs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_concept_preferences (user_id, concept_id)
				VALUES ($1, $2)
			`, userID, conceptID); err != nil {
				return fmt.Errorf("insert concept preference: %w", err)
			}
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET onboarding_completed = true,
		    updated_at = NOW()
		WHERE id = $1
	`, userID); err != nil {
		return fmt.Errorf("update onboarding state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (db *DB) RefreshTodayRecommendations(ctx context.Context, userID string) (time.Time, []Recommendation, error) {
	conceptCodes, err := db.userConceptCodes(ctx, userID)
	if err != nil {
		return time.Time{}, nil, err
	}
	tagFilters := tagsForConceptCodes(conceptCodes)

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userTZ := "America/New_York"
	if err := tx.QueryRow(ctx, `SELECT timezone FROM users WHERE id = $1`, userID).Scan(&userTZ); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, nil, ErrUserNotFound
		}
		return time.Time{}, nil, fmt.Errorf("load user timezone: %w", err)
	}
	loc, err := time.LoadLocation(userTZ)
	if err != nil {
		loc = time.UTC
	}
	today := startOfDayInLocation(time.Now(), loc)
	completedPositions := make(map[int16]struct{})
	rows, err := tx.Query(ctx, `
        SELECT position
        FROM daily_assignments
        WHERE user_id = $1
          AND assignment_date = $2
          AND status = 'COMPLETED'
    `, userID, today)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("query completed positions: %w", err)
	}
	for rows.Next() {
		var pos int16
		if scanErr := rows.Scan(&pos); scanErr != nil {
			rows.Close()
			return time.Time{}, nil, fmt.Errorf("scan completed position: %w", scanErr)
		}
		completedPositions[pos] = struct{}{}
	}
	rows.Close()
	if rows.Err() != nil {
		return time.Time{}, nil, fmt.Errorf("iterate completed positions: %w", rows.Err())
	}

	if _, err := tx.Exec(ctx, `
        DELETE FROM daily_assignments
        WHERE user_id = $1
          AND assignment_date = $2
          AND status = 'ASSIGNED'
    `, userID, today); err != nil {
		return time.Time{}, nil, fmt.Errorf("delete existing assignments: %w", err)
	}

	positionToFill := make([]int16, 0, dailyLimit)
	for i := 1; i <= dailyLimit; i++ {
		pos := int16(i)
		if _, taken := completedPositions[pos]; taken {
			continue
		}
		positionToFill = append(positionToFill, pos)
	}

	if len(positionToFill) == 0 {
		assignments, err := queryAssignments(ctx, tx, userID, today)
		if err != nil {
			return time.Time{}, nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return time.Time{}, nil, fmt.Errorf("commit refresh tx: %w", err)
		}
		return today, assignments, nil
	}

	candidates, err := queryQueueCandidates(ctx, tx, userID, len(positionToFill), tagFilters)
	if err != nil {
		return time.Time{}, nil, err
	}

	for i, c := range candidates {
		if i >= len(positionToFill) {
			break
		}
		position := positionToFill[i]
		if _, err := tx.Exec(ctx, `
            INSERT INTO daily_assignments (user_id, problem_id, assignment_date, position, status)
            VALUES ($1, $2, $3, $4, 'ASSIGNED')
            ON CONFLICT (user_id, assignment_date, position) DO NOTHING
        `, userID, c.ProblemID, today, position); err != nil {
			return time.Time{}, nil, fmt.Errorf("insert refreshed assignment: %w", err)
		}
	}

	assignments, err := queryAssignments(ctx, tx, userID, today)
	if err != nil {
		return time.Time{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, nil, fmt.Errorf("commit refresh tx: %w", err)
	}
	return today, assignments, nil
}

func (db *DB) ListCompletedProblems(ctx context.Context, userID string, limit int) ([]CompletedProblem, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := db.pool.Query(ctx, `
		SELECT
			p.id,
			p.slug,
			p.title,
			p.url,
			p.difficulty,
			pc.completed_at,
			COALESCE(pc.submission_url, '')
		FROM problem_completions pc
		JOIN problems p ON p.id = pc.problem_id
		WHERE pc.user_id = $1
		ORDER BY pc.completed_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query completed problems: %w", err)
	}
	defer rows.Close()

	out := make([]CompletedProblem, 0, limit)
	for rows.Next() {
		var p CompletedProblem
		var completedAt time.Time
		if err := rows.Scan(
			&p.ProblemID,
			&p.Slug,
			&p.Title,
			&p.URL,
			&p.Difficulty,
			&completedAt,
			&p.SubmissionURL,
		); err != nil {
			return nil, fmt.Errorf("scan completion row: %w", err)
		}
		p.CompletedAt = completedAt.UTC().Format(time.RFC3339)
		out = append(out, p)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate completion rows: %w", rows.Err())
	}
	return out, nil
}

func (db *DB) userConceptCodes(ctx context.Context, userID string) ([]string, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT c.code
		FROM user_concept_preferences ucp
		JOIN concepts c ON c.id = ucp.concept_id
		WHERE ucp.user_id = $1
		ORDER BY c.display_name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query user concepts: %w", err)
	}
	defer rows.Close()

	codes := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan user concept code: %w", err)
		}
		codes = append(codes, code)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate user concept codes: %w", rows.Err())
	}
	return codes, nil
}

func VerifyLeetCodeUsername(ctx context.Context, httpClient *http.Client, username string) (bool, error) {
	username = strings.TrimSpace(username)
	if !leetCodeUsernamePattern.MatchString(username) {
		return false, nil
	}

	query := `
query userPublicProfile($username: String!) {
  matchedUser(username: $username) {
    username
  }
}
`

	body, err := json.Marshal(map[string]any{
		"query": query,
		"variables": map[string]any{
			"username": username,
		},
	})
	if err != nil {
		return false, fmt.Errorf("marshal leetcode graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://leetcode.com/graphql", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("create leetcode graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://leetcode.com")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("send leetcode graphql request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("leetcode graphql status=%d", resp.StatusCode)
	}

	var payload struct {
		Data struct {
			MatchedUser *struct {
				Username string `json:"username"`
			} `json:"matchedUser"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("decode leetcode graphql response: %w", err)
	}
	if payload.Data.MatchedUser == nil {
		return false, nil
	}
	return strings.EqualFold(payload.Data.MatchedUser.Username, username), nil
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes for uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	hexBytes := make([]byte, 36)
	hex.Encode(hexBytes[0:8], b[0:4])
	hexBytes[8] = '-'
	hex.Encode(hexBytes[9:13], b[4:6])
	hexBytes[13] = '-'
	hex.Encode(hexBytes[14:18], b[6:8])
	hexBytes[18] = '-'
	hex.Encode(hexBytes[19:23], b[8:10])
	hexBytes[23] = '-'
	hex.Encode(hexBytes[24:36], b[10:16])
	return string(hexBytes), nil
}
