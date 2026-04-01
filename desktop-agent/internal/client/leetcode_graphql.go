package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const leetCodeGraphQLEndpoint = "https://leetcode.com/graphql"

type LeetCodeSubmission struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	TitleSlug string `json:"titleSlug"`
	Timestamp string `json:"timestamp"`
}

type leetCodeGraphQLError struct {
	Message string `json:"message"`
}

func FetchRecentACSubmissions(ctx context.Context, httpClient *http.Client, username string, limit int) ([]LeetCodeSubmission, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("leetcode username is required")
	}
	if limit <= 0 {
		limit = 20
	}

	query := `query RecentAcSubmissions($username: String!, $limit: Int!) {
  recentAcSubmissionList(username: $username, limit: $limit) {
    id
    title
    titleSlug
    timestamp
  }
}`

	payload, err := json.Marshal(map[string]any{
		"query": query,
		"variables": map[string]any{
			"username": username,
			"limit":    limit,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode leetcode history query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, leetCodeGraphQLEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build leetcode history request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var out struct {
		Data struct {
			RecentAcSubmissionList []LeetCodeSubmission `json:"recentAcSubmissionList"`
		} `json:"data"`
		Errors []leetCodeGraphQLError `json:"errors"`
	}
	if err := doGraphQL(httpClient, req, &out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 && strings.TrimSpace(out.Errors[0].Message) != "" {
		return nil, fmt.Errorf("leetcode graphql error: %s", strings.TrimSpace(out.Errors[0].Message))
	}
	return out.Data.RecentAcSubmissionList, nil
}

func ResolveProblemFrontendID(ctx context.Context, httpClient *http.Client, titleSlug string) (int64, error) {
	titleSlug = strings.TrimSpace(titleSlug)
	if titleSlug == "" {
		return 0, fmt.Errorf("title slug is required")
	}

	query := `query ResolveProblemId($titleSlug: String!) {
  question(titleSlug: $titleSlug) {
    questionFrontendId
  }
}`
	payload, err := json.Marshal(map[string]any{
		"query": query,
		"variables": map[string]any{
			"titleSlug": titleSlug,
		},
	})
	if err != nil {
		return 0, fmt.Errorf("encode leetcode resolve query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, leetCodeGraphQLEndpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("build leetcode resolve request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var out struct {
		Data struct {
			Question struct {
				QuestionFrontendID string `json:"questionFrontendId"`
			} `json:"question"`
		} `json:"data"`
		Errors []leetCodeGraphQLError `json:"errors"`
	}
	if err := doGraphQL(httpClient, req, &out); err != nil {
		return 0, err
	}
	if len(out.Errors) > 0 && strings.TrimSpace(out.Errors[0].Message) != "" {
		return 0, fmt.Errorf("leetcode graphql error: %s", strings.TrimSpace(out.Errors[0].Message))
	}

	id, err := strconv.ParseInt(strings.TrimSpace(out.Data.Question.QuestionFrontendID), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid frontend question id returned for slug %q", titleSlug)
	}
	return id, nil
}

func doGraphQL(httpClient *http.Client, req *http.Request, out any) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s failed: %w", req.URL.String(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("request %s status=%d", req.URL.String(), resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode graphql response: %w", err)
	}
	return nil
}
