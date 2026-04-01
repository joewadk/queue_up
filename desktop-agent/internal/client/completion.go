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

type dailyQueueResponse struct {
	Queue []dailyQueueItem `json:"queue"`
}

type dailyQueueItem struct {
	ProblemID   int64 `json:"problem_id"`
	IsCompleted bool  `json:"is_completed"`
}

type markCompletionRequest struct {
	UserID       string `json:"user_id"`
	ProblemID    int64  `json:"problem_id"`
	Source       string `json:"source"`
	Verification string `json:"verification"`
}

func MarkFirstIncompleteToday(ctx context.Context, httpClient *http.Client, baseURL, userID string) (int64, error) {
	if strings.TrimSpace(baseURL) == "" {
		return 0, fmt.Errorf("backend base url is empty")
	}
	if strings.TrimSpace(userID) == "" {
		return 0, fmt.Errorf("user_id is empty")
	}

	problemID, err := fetchFirstIncompleteProblemID(ctx, httpClient, baseURL, userID)
	if err != nil {
		rec, recErr := FetchTodayRecommendation(ctx, httpClient, baseURL, userID)
		if recErr != nil {
			return 0, fmt.Errorf("resolve problem id from queue (%v) and recommendation (%v)", err, recErr)
		}
		if rec.ProblemID <= 0 {
			return 0, fmt.Errorf("recommended problem_id is invalid")
		}
		problemID = rec.ProblemID
	}

	if err := postCompletion(ctx, httpClient, baseURL, markCompletionRequest{
		UserID:       userID,
		ProblemID:    problemID,
		Source:       "desktop",
		Verification: "manual",
	}); err != nil {
		return 0, err
	}

	return problemID, nil
}

func fetchFirstIncompleteProblemID(ctx context.Context, httpClient *http.Client, baseURL, userID string) (int64, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return 0, fmt.Errorf("parse backend base url: %w", err)
	}
	u.Path = "/v1/daily-queue"
	q := u.Query()
	q.Set("user_id", userID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("build daily-queue request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request daily-queue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("daily-queue status=%d", resp.StatusCode)
	}

	var payload dailyQueueResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, fmt.Errorf("decode daily-queue: %w", err)
	}
	if len(payload.Queue) == 0 {
		return 0, fmt.Errorf("today queue is empty")
	}
	for _, item := range payload.Queue {
		if item.ProblemID <= 0 {
			continue
		}
		if !item.IsCompleted {
			return item.ProblemID, nil
		}
	}
	if payload.Queue[0].ProblemID <= 0 {
		return 0, fmt.Errorf("queue problem_id is invalid")
	}
	return payload.Queue[0].ProblemID, nil
}

func postCompletion(ctx context.Context, httpClient *http.Client, baseURL string, reqBody markCompletionRequest) error {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return fmt.Errorf("parse backend base url: %w", err)
	}
	u.Path = "/v1/completions"

	b, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("encode completion payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("build completion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request completion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("completion status=%d", resp.StatusCode)
	}
	return nil
}
