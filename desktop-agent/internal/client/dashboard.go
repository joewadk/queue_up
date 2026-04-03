package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type ConceptsResponse struct {
	Concepts []Concept `json:"concepts"`
}

type Concept struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
}

type UserLookupResponse struct {
	Exists   bool        `json:"exists"`
	Username string      `json:"username"`
	Profile  UserProfile `json:"profile"`
}

type BootstrapResponse struct {
	Created            bool        `json:"created"`
	Profile            UserProfile `json:"profile"`
	VerificationStatus string      `json:"verification_status"`
}

type UserProfile struct {
	UserID             string   `json:"user_id"`
	LeetCodeUsername   string   `json:"leetcode_username"`
	Timezone           string   `json:"timezone"`
	OnboardingComplete bool     `json:"onboarding_complete"`
	ConceptCodes       []string `json:"concept_codes"`
}

type RefreshQueueResponse struct {
	Recommendations []Recommendation `json:"recommendations"`
}

type HistoryResponse struct {
	History []CompletedProblem `json:"history"`
}

type DailyQueueResponse struct {
	Count          int              `json:"count"`
	CompletedCount int              `json:"completed_count"`
	Queue          []DailyQueueItem `json:"queue"`
}

type ProblemQueueResult struct {
	Recommendations []Recommendation
	Source          string
}

type DailyQueueItem struct {
	Position    int16  `json:"position"`
	ProblemID   int64  `json:"problem_id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Difficulty  string `json:"difficulty"`
	IsCompleted bool   `json:"is_completed"`
}

type CompletedProblem struct {
	ProblemID     int64  `json:"problem_id"`
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	Difficulty    string `json:"difficulty"`
	CompletedAt   string `json:"completed_at"`
	SubmissionURL string `json:"submission_url"`
}

func LookupUserByLeetCode(ctx context.Context, httpClient *http.Client, baseURL, username string) (UserLookupResponse, error) {
	u, err := parseBase(baseURL, "/v1/users/by-leetcode")
	if err != nil {
		return UserLookupResponse{}, err
	}
	q := u.Query()
	q.Set("username", username)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return UserLookupResponse{}, fmt.Errorf("build lookup request: %w", err)
	}
	var out UserLookupResponse
	if err := doJSON(httpClient, req, &out); err != nil {
		return UserLookupResponse{}, err
	}
	return out, nil
}

func BootstrapUser(ctx context.Context, httpClient *http.Client, baseURL, username, timezone string) (BootstrapResponse, error) {
	u, err := parseBase(baseURL, "/v1/users/bootstrap")
	if err != nil {
		return BootstrapResponse{}, err
	}

	payload, err := json.Marshal(map[string]any{
		"leetcode_username": username,
		"timezone":          timezone,
		"verify_username":   false,
	})
	if err != nil {
		return BootstrapResponse{}, fmt.Errorf("encode bootstrap payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return BootstrapResponse{}, fmt.Errorf("build bootstrap request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var out BootstrapResponse
	if err := doJSON(httpClient, req, &out); err != nil {
		return BootstrapResponse{}, err
	}
	return out, nil
}

func FetchConcepts(ctx context.Context, httpClient *http.Client, baseURL string) (ConceptsResponse, error) {
	u, err := parseBase(baseURL, "/v1/concepts")
	if err != nil {
		return ConceptsResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ConceptsResponse{}, fmt.Errorf("build concepts request: %w", err)
	}
	var out ConceptsResponse
	if err := doJSON(httpClient, req, &out); err != nil {
		return ConceptsResponse{}, err
	}
	return out, nil
}

func UpdateConcepts(ctx context.Context, httpClient *http.Client, baseURL, userID string, conceptCodes []string) error {
	u, err := parseBase(baseURL, "/v1/users/"+strings.TrimSpace(userID)+"/concepts")
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"concept_codes": conceptCodes,
	})
	if err != nil {
		return fmt.Errorf("encode update concepts payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build update concepts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return doJSON(httpClient, req, nil)
}

func RefreshQueue(ctx context.Context, httpClient *http.Client, baseURL, userID string) (RefreshQueueResponse, error) {
	u, err := parseBase(baseURL, "/v1/users/"+strings.TrimSpace(userID)+"/queue/refresh")
	if err != nil {
		return RefreshQueueResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return RefreshQueueResponse{}, fmt.Errorf("build refresh queue request: %w", err)
	}
	var out RefreshQueueResponse
	if err := doJSON(httpClient, req, &out); err != nil {
		return RefreshQueueResponse{}, err
	}
	return out, nil
}

func FetchProblemQueue(ctx context.Context, httpClient *http.Client, baseURL, userID, selectedConceptCode string) (ProblemQueueResult, error) {
	refreshOut, refreshErr := RefreshQueue(ctx, httpClient, baseURL, userID)
	if refreshErr == nil && len(refreshOut.Recommendations) > 0 {
		return ProblemQueueResult{
			Recommendations: refreshOut.Recommendations,
			Source:          "refresh endpoint",
		}, nil
	}
	if refreshErr == nil && len(refreshOut.Recommendations) == 0 && strings.TrimSpace(selectedConceptCode) != "" {
		return ProblemQueueResult{
			Recommendations: []Recommendation{},
			Source:          "selected concept empty",
		}, nil
	}

	dailyQueueOut, dailyQueueErr := FetchDailyQueue(ctx, httpClient, baseURL, userID)
	if dailyQueueErr == nil && len(dailyQueueOut.Queue) > 0 {
		dailyRecs := make([]Recommendation, 0, len(dailyQueueOut.Queue))
		for _, q := range dailyQueueOut.Queue {
			dailyRecs = append(dailyRecs, Recommendation{
				ProblemID:  q.ProblemID,
				Slug:       q.Slug,
				Title:      q.Title,
				URL:        q.URL,
				Difficulty: q.Difficulty,
			})
		}
		return ProblemQueueResult{
			Recommendations: dailyRecs,
			Source:          "daily queue fallback",
		}, nil
	}

	todayFallback, todayFallbackErr := FetchTodayRecommendations(ctx, httpClient, baseURL, userID)
	if todayFallbackErr == nil && len(todayFallback) > 0 {
		return ProblemQueueResult{
			Recommendations: todayFallback,
			Source:          "today recommendation fallback",
		}, nil
	}

	if refreshErr != nil {
		return ProblemQueueResult{}, fmt.Errorf("queue refresh failed: %w", refreshErr)
	}
	if dailyQueueErr != nil {
		return ProblemQueueResult{}, fmt.Errorf("queue daily fallback failed: %w", dailyQueueErr)
	}
	if todayFallbackErr != nil {
		return ProblemQueueResult{}, fmt.Errorf("queue fallback failed: %w", todayFallbackErr)
	}
	return ProblemQueueResult{
		Recommendations: []Recommendation{},
		Source:          "empty",
	}, nil
}

func FetchDailyQueue(ctx context.Context, httpClient *http.Client, baseURL, userID string) (DailyQueueResponse, error) {
	u, err := parseBase(baseURL, "/v1/daily-queue")
	if err != nil {
		return DailyQueueResponse{}, err
	}
	q := u.Query()
	q.Set("user_id", strings.TrimSpace(userID))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return DailyQueueResponse{}, fmt.Errorf("build daily queue request: %w", err)
	}
	var out DailyQueueResponse
	if err := doJSON(httpClient, req, &out); err != nil {
		return DailyQueueResponse{}, err
	}
	return out, nil
}

func FetchHistory(ctx context.Context, httpClient *http.Client, baseURL, userID string, limit int) (HistoryResponse, error) {
	u, err := parseBase(baseURL, "/v1/users/"+strings.TrimSpace(userID)+"/history")
	if err != nil {
		return HistoryResponse{}, err
	}
	q := u.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return HistoryResponse{}, fmt.Errorf("build history request: %w", err)
	}
	var out HistoryResponse
	if err := doJSON(httpClient, req, &out); err != nil {
		return HistoryResponse{}, err
	}
	return out, nil
}

func MarkCompletion(ctx context.Context, httpClient *http.Client, baseURL, userID string, problemID int64, submissionURL string) error {
	u, err := parseBase(baseURL, "/v1/completions")
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"user_id":        userID,
		"problem_id":     problemID,
		"source":         "desktop",
		"verification":   "manual",
		"submission_url": submissionURL,
	})
	if err != nil {
		return fmt.Errorf("encode mark completion payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build mark completion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return doJSON(httpClient, req, nil)
}

func parseBase(baseURL, path string) (*url.URL, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("backend base url is empty")
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse backend base url: %w", err)
	}
	u.Path = path
	return u, nil
}

func doJSON(httpClient *http.Client, req *http.Request, out any) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if msg, ok := apiErr["error"].(string); ok && strings.TrimSpace(msg) != "" {
			return fmt.Errorf("%s %s failed: %s", req.Method, req.URL.Path, msg)
		}
		return fmt.Errorf("%s %s status=%d", req.Method, req.URL.Path, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s %s response: %w", req.Method, req.URL.Path, err)
	}
	return nil
}
