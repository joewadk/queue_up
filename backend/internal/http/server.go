package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"queue_up/backend/internal/store"
)

func New(db *store.DB) http.Handler {
	mux := http.NewServeMux()
	leetCodeClient := &http.Client{Timeout: 7 * time.Second}

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

	mux.HandleFunc("/v1/concepts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		concepts, err := db.ListConcepts(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"count":    len(concepts),
			"concepts": concepts,
		})
	})

	mux.HandleFunc("/v1/users/by-leetcode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		username := strings.TrimSpace(r.URL.Query().Get("username"))
		if username == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing required query param: username"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		profile, found, err := db.GetUserByLeetCodeUsername(ctx, username)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"exists":   found,
			"username": username,
			"profile":  profile,
		})
	})

	mux.HandleFunc("/v1/users/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		var req bootstrapRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
			return
		}
		req.LeetCodeUsername = strings.TrimSpace(req.LeetCodeUsername)
		if req.LeetCodeUsername == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "leetcode_username is required"})
			return
		}

		verify := false
		if req.VerifyUsername != nil {
			verify = *req.VerifyUsername
		}
		verificationStatus := "skipped"
		if verify {
			ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
			valid, err := store.VerifyLeetCodeUsername(ctx, leetCodeClient, req.LeetCodeUsername)
			cancel()
			if err != nil {
				verificationStatus = "unavailable"
			} else if !valid {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "leetcode username does not exist"})
				return
			} else {
				verificationStatus = "verified"
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		profile, created, err := db.BootstrapUser(ctx, req.LeetCodeUsername, req.Timezone)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created":             created,
			"profile":             profile,
			"verification_status": verificationStatus,
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
			"recommendation_count":  len(recs),
			"recommendations":       recs,
			"recommendation_source": "easy_first_by_concept",
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
			"status":       "ok",
			"user_id":      req.UserID,
			"problem_id":   req.ProblemID,
			"completed_at": completedAt.Format(time.RFC3339),
			"source":       req.Source,
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

	mux.HandleFunc("/v1/users/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v1/users/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}

		userID := strings.TrimSpace(parts[0])
		resource := parts[1]

		switch resource {
		case "concepts":
			if r.Method != http.MethodPut {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
				return
			}
			var req setConceptsRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			if err := db.SetUserConceptPreferences(ctx, userID, req.ConceptCodes); err != nil {
				if errors.Is(err, store.ErrUserNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
					return
				}
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
				return
			}
			date, recs, err := db.RefreshTodayRecommendations(ctx, userID)
			if err != nil {
				if errors.Is(err, store.ErrUserNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
					return
				}
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status":               "ok",
				"user_id":              userID,
				"concept_codes":        req.ConceptCodes,
				"assignment_date":      date.Format("2006-01-02"),
				"recommendation_count": len(recs),
				"recommendations":      recs,
			})
			return

		case "queue":
			if len(parts) != 3 || parts[2] != "refresh" {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
				return
			}
			if r.Method != http.MethodPost {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			date, recs, err := db.RefreshTodayRecommendations(ctx, userID)
			if err != nil {
				if errors.Is(err, store.ErrUserNotFound) {
					writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
					return
				}
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status":               "ok",
				"user_id":              userID,
				"assignment_date":      date.Format("2006-01-02"),
				"recommendation_count": len(recs),
				"recommendations":      recs,
			})
			return

		case "history":
			if r.Method != http.MethodGet {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
				return
			}
			limit := 25
			if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
				if parsed, err := strconv.Atoi(rawLimit); err == nil {
					limit = parsed
				}
			}
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			items, err := db.ListCompletedProblems(ctx, userID, limit)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"user_id": userID,
				"count":   len(items),
				"history": items,
			})
			return
		}

		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
	})

	return mux
}

type completionRequest struct {
	UserID        string `json:"user_id"`
	ProblemID     int64  `json:"problem_id"`
	Timestamp     string `json:"timestamp"`
	Source        string `json:"source"`
	Verification  string `json:"verification"`
	SubmissionURL string `json:"submission_url"`
}

type bootstrapRequest struct {
	LeetCodeUsername string `json:"leetcode_username"`
	Timezone         string `json:"timezone"`
	VerifyUsername   *bool  `json:"verify_username"`
}

type setConceptsRequest struct {
	ConceptCodes []string `json:"concept_codes"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
