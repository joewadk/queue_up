package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"queue_up/backend/internal/store"
)
//handler for all http endpoints. takes in a db connection and returns an http.Handler
func New(db *store.DB) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "degraded",
				"db":     "down",
				"error":  err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"db":     "up",
		})
	})

	mux.HandleFunc("/v1/recommendation/today", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}

		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "missing required query param: user_id",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		date, recs, err := db.GetOrCreateTodayRecommendations(ctx, userID)
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{
					"error":   "user not found",
					"user_id": userID,
				})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"user_id":               userID,
			"assignment_date":       date.Format("2006-01-02"),
			"daily_cap":             3,
			"recommendation_count": len(recs),
			"recommendations":       recs,
		})
	})

	mux.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}

		var req completionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
			return
		}
		if req.UserID == "" || req.ProblemID <= 0 || req.Source == "" || req.Verification == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "user_id, problem_id, source, and verification are required",
			})
			return
		}

		completedAt := time.Now().UTC()
		if req.Timestamp != "" {
			parsed, err := time.Parse(time.RFC3339, req.Timestamp)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "timestamp must be RFC3339"})
				return
			}
			completedAt = parsed.UTC()
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		err := db.RecordCompletion(ctx, store.CompletionInput{
			UserID:        req.UserID,
			ProblemID:     req.ProblemID,
			CompletedAt:   completedAt,
			Source:        req.Source,
			Verification:  req.Verification,
			SubmissionURL: req.SubmissionURL,
		})
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status":      "ok",
			"user_id":     req.UserID,
			"problem_id":  req.ProblemID,
			"completed_at": completedAt.Format(time.RFC3339),
			"source":      req.Source,
			"verification": req.Verification,
		})
	})

	mux.HandleFunc("/v1/daily-queue", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}

		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing required query param: user_id"})
			return
		}

		var targetDate *time.Time
		if dateStr := r.URL.Query().Get("date"); dateStr != "" {
			parsed, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "date must be YYYY-MM-DD"})
				return
			}
			targetDate = &parsed
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		date, queue, err := db.GetDailyQueue(ctx, userID, targetDate)
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		completed := 0
		for _, q := range queue {
			if q.IsCompleted {
				completed++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"user_id":         userID,
			"date":            date.Format("2006-01-02"),
			"count":           len(queue),
			"completed_count": completed,
			"queue":           queue,
		})
	})

	return mux
}

type completionRequest struct {
	UserID       string `json:"user_id"`
	ProblemID    int64  `json:"problem_id"`
	Timestamp    string `json:"timestamp"`
	Source       string `json:"source"`
	Verification string `json:"verification"`
	SubmissionURL string `json:"submission_url"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
