package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type recommendationResponse struct {
	Recommendations []recommendationItem `json:"recommendations"`
}

type recommendationItem struct {
	ProblemID int64  `json:"problem_id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	URL       string `json:"url"`
}

type Recommendation struct {
	ProblemID int64
	Slug      string
	Title     string
	URL       string
}

func FetchTodayRecommendation(ctx context.Context, httpClient *http.Client, baseURL, userID string) (Recommendation, error) {
	if strings.TrimSpace(baseURL) == "" {
		return Recommendation{}, fmt.Errorf("backend base url is empty")
	}
	if strings.TrimSpace(userID) == "" {
		return Recommendation{}, fmt.Errorf("user_id is empty")
	}

	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return Recommendation{}, fmt.Errorf("parse backend base url: %w", err)
	}
	u.Path = "/v1/recommendation/today"
	q := u.Query()
	q.Set("user_id", userID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Recommendation{}, fmt.Errorf("build recommendation request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return Recommendation{}, fmt.Errorf("request recommendation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Recommendation{}, fmt.Errorf("recommendation status=%d", resp.StatusCode)
	}

	var payload recommendationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Recommendation{}, fmt.Errorf("decode recommendation: %w", err)
	}
	if len(payload.Recommendations) == 0 {
		return Recommendation{}, fmt.Errorf("no recommendations returned")
	}

	first := payload.Recommendations[0]
	if strings.TrimSpace(first.URL) == "" {
		return Recommendation{}, fmt.Errorf("recommended URL is empty")
	}
	return Recommendation{
		ProblemID: first.ProblemID,
		Slug:      strings.TrimSpace(first.Slug),
		Title:     strings.TrimSpace(first.Title),
		URL:       strings.TrimSpace(first.URL),
	}, nil
}
