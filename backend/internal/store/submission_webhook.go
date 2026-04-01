package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type submissionSanitizerWebhookRequest struct {
	UserID        string `json:"user_id"`
	ProblemID     int64  `json:"problem_id"`
	ExpectedSlug  string `json:"expected_slug"`
	SubmissionURL string `json:"submission_url"`
}

type submissionSanitizerWebhookResponse struct {
	Valid                  bool   `json:"valid"`
	SanitizedSubmissionURL string `json:"sanitized_submission_url"`
	Reason                 string `json:"reason"`
}

func (db *DB) sanitizeSubmissionURLViaWebhook(ctx context.Context, userID string, problemID int64, expectedSlug, rawURL string) (string, error) {
	webhookURL := strings.TrimSpace(db.submissionSanitizerWebhookURL)
	if webhookURL == "" {
		return "", errSubmissionSanitizerWebhookDisabled
	}
	client := db.submissionSanitizerHTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	payload, err := json.Marshal(submissionSanitizerWebhookRequest{
		UserID:        strings.TrimSpace(userID),
		ProblemID:     problemID,
		ExpectedSlug:  strings.TrimSpace(expectedSlug),
		SubmissionURL: strings.TrimSpace(rawURL),
	})
	if err != nil {
		return "", fmt.Errorf("encode submission sanitizer webhook request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build submission sanitizer webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("submission sanitizer webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("submission sanitizer webhook status=%d", resp.StatusCode)
	}

	var out submissionSanitizerWebhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode submission sanitizer webhook response: %w", err)
	}
	if !out.Valid {
		msg := strings.TrimSpace(out.Reason)
		if msg == "" {
			msg = "submission URL rejected by sanitizer webhook"
		}
		return "", fmt.Errorf(msg)
	}
	sanitized := strings.TrimSpace(out.SanitizedSubmissionURL)
	if sanitized == "" {
		return "", fmt.Errorf("submission sanitizer webhook returned empty sanitized_submission_url")
	}
	return sanitized, nil
}
