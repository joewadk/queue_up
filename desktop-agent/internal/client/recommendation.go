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
	ProblemID  int64  `json:"problem_id"`
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Difficulty string `json:"difficulty"`
}

type Recommendation struct {
	ProblemID  int64
	Slug       string
	Title      string
	URL        string
	Difficulty string
}

func FetchTodayRecommendations(ctx context.Context, httpClient *http.Client, baseURL, userID string) ([]Recommendation, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("backend base url is empty")
	}
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user_id is empty")
	}

	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse backend base url: %w", err)
	}
	u.Path = "/v1/recommendation/today"
	q := u.Query()
	q.Set("user_id", userID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build recommendation request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request recommendation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("recommendation status=%d", resp.StatusCode)
	}

	var payload recommendationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode recommendation: %w", err)
	}
	if len(payload.Recommendations) == 0 {
		return nil, fmt.Errorf("no recommendations returned")
	}

	out := make([]Recommendation, 0, len(payload.Recommendations))
	for _, item := range payload.Recommendations {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		out = append(out, Recommendation{
			ProblemID:  item.ProblemID,
			Slug:       strings.TrimSpace(item.Slug),
			Title:      strings.TrimSpace(item.Title),
			URL:        strings.TrimSpace(item.URL),
			Difficulty: strings.TrimSpace(item.Difficulty),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("recommended URLs are empty")
	}
	return out, nil
}

func FetchTodayRecommendation(ctx context.Context, httpClient *http.Client, baseURL, userID string) (Recommendation, error) {
	recs, err := FetchTodayRecommendations(ctx, httpClient, baseURL, userID)
	if err != nil {
		return Recommendation{}, err
	}
	return recs[0], nil
}
