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
	"net/url"
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

	prefIDs, err := queryPreferredConceptIDs(ctx, tx, userID)
	if err != nil {
		return time.Time{}, nil, err
	}
	positionToFill := make([]int16, 0, dailyLimit)
	for i := 1; i <= dailyLimit; i++ {
		pos := int16(i)
		if _, taken := completedPositions[pos]; taken {
			continue
		}
		positionToFill = append(positionToFill, pos)
	}

	candidates, err := queryCandidates(ctx, tx, userID, prefIDs)
	if err != nil {
		return time.Time{}, nil, err
	}
	if len(candidates) < len(positionToFill) {
		if seedErr := db.seedLeetCodeAPIFallback(ctx, tx, prefIDs); seedErr == nil {
			refreshed, requeryErr := queryCandidates(ctx, tx, userID, prefIDs)
			if requeryErr == nil {
				candidates = refreshed
			}
		}
	}
	// Backfill from global candidates if preferred-category inventory is short.
	if len(candidates) < len(positionToFill) && len(prefIDs) > 0 {
		fallbackCandidates, fallbackErr := queryCandidates(ctx, tx, userID, nil)
		if fallbackErr == nil {
			candidates = mergeUniqueRecommendations(candidates, fallbackCandidates, len(positionToFill))
		}
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

type leetCodeTagResponse struct {
	Tag      string `json:"tag"`
	Problems []struct {
		Title      string `json:"title"`
		TitleSlug  string `json:"title_slug"`
		URL        string `json:"url"`
		Difficulty string `json:"difficulty"`
		PaidOnly   bool   `json:"paid_only"`
	} `json:"problems"`
}

func (db *DB) seedLeetCodeAPIFallback(ctx context.Context, tx pgx.Tx, prefIDs []int64) error {
	baseURL := strings.TrimSpace(db.leetCodeAPIBaseURL)
	if baseURL == "" {
		return fmt.Errorf("leetcode api base url is empty")
	}
	httpClient := db.leetCodeAPIHTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 4 * time.Second}
	}

	conceptRows, err := tx.Query(ctx, `
		SELECT id, code
		FROM concepts
		WHERE id = ANY($1)
	`, prefIDs)
	if err != nil {
		return fmt.Errorf("query preferred concept codes: %w", err)
	}
	type conceptTag struct {
		id      int64
		apiSlug string
	}
	var targets []conceptTag
	for conceptRows.Next() {
		var id int64
		var code string
		if scanErr := conceptRows.Scan(&id, &code); scanErr != nil {
			conceptRows.Close()
			return fmt.Errorf("scan preferred concept code: %w", scanErr)
		}
		tag, ok := conceptCodeToLeetCodeTag(strings.TrimSpace(code))
		if !ok {
			continue
		}
		targets = append(targets, conceptTag{id: id, apiSlug: tag})
	}
	conceptRows.Close()
	if conceptRows.Err() != nil {
		return fmt.Errorf("iterate preferred concept codes: %w", conceptRows.Err())
	}
	if len(targets) == 0 {
		return fmt.Errorf("no preferred concept tags to seed")
	}

	for _, target := range targets {
		endpoint := strings.TrimRight(baseURL, "/") + "/problems/tag/" + url.PathEscape(target.apiSlug)
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if reqErr != nil {
			return fmt.Errorf("build leetcode api request: %w", reqErr)
		}
		resp, httpErr := httpClient.Do(req)
		if httpErr != nil {
			return fmt.Errorf("leetcode api request failed: %w", httpErr)
		}
		var payload leetCodeTagResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
			resp.Body.Close()
			return fmt.Errorf("decode leetcode api response: %w", decodeErr)
		}
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("leetcode api status=%d", resp.StatusCode)
		}

		inserted := 0
		for _, p := range payload.Problems {
			if p.PaidOnly {
				continue
			}
			slug := strings.TrimSpace(p.TitleSlug)
			title := strings.TrimSpace(p.Title)
			problemURL := strings.TrimSpace(p.URL)
			diff := normalizeDifficulty(strings.TrimSpace(p.Difficulty))
			if slug == "" || title == "" || problemURL == "" {
				continue
			}
			if _, execErr := tx.Exec(ctx, `
				INSERT INTO problems (slug, title, difficulty, url, source_set, active)
				VALUES ($1, $2, $3, $4, 'LEETCODE_API', true)
				ON CONFLICT (slug) DO UPDATE
				SET title = EXCLUDED.title,
				    difficulty = EXCLUDED.difficulty,
				    url = EXCLUDED.url,
				    active = true
			`, slug, title, diff, problemURL); execErr != nil {
				return fmt.Errorf("upsert leetcode api problem: %w", execErr)
			}
			if _, execErr := tx.Exec(ctx, `
				INSERT INTO problem_concepts (problem_id, concept_id)
				SELECT p.id, $2
				FROM problems p
				WHERE p.slug = $1
				ON CONFLICT DO NOTHING
			`, slug, target.id); execErr != nil {
				return fmt.Errorf("upsert leetcode api concept map: %w", execErr)
			}
			inserted++
			if inserted >= 30 {
				break
			}
		}
	}
	return nil
}

func conceptCodeToLeetCodeTag(code string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "ARRAY":
		return "array", true
	case "TWO_POINTER":
		return "two-pointers", true
	case "STACK":
		return "stack", true
	case "BINARY_SEARCH":
		return "binary-search", true
	case "SLIDING_WINDOW":
		return "sliding-window", true
	case "LINKED_LIST":
		return "linked-list", true
	case "TREE":
		return "tree", true
	case "HEAP_PRIORITY_QUEUE":
		return "heap-priority-queue", true
	case "BACKTRACKING":
		return "backtracking", true
	case "TRIE":
		return "trie", true
	case "INTERVAL":
		return "interval", true
	case "GREEDY":
		return "greedy", true
	case "BIT_MANIPULATION":
		return "bit-manipulation", true
	case "PREFIX_SUM":
		return "prefix-sum", true
	case "GRAPH":
		return "graph", true
	case "DP":
		return "dynamic-programming", true
	case "DFS":
		return "depth-first-search", true
	case "BFS":
		return "breadth-first-search", true
	case "QUEUE":
		return "queue", true
	default:
		return "", false
	}
}

func normalizeDifficulty(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "easy":
		return "Easy"
	case "hard":
		return "Hard"
	case "medium":
		return "Medium"
	default:
		return "Medium"
	}
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
