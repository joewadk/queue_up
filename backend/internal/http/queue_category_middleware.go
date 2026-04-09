package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"queue_up/backend/internal/store"
)

type userIDExtractor func(*http.Request) string

func withQueueCategoryGuard(db *store.DB, extract userIDExtractor, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(extract(r))
		if userID != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			err := db.EnsureTodayQueueMatchesCategory(ctx, userID)
			cancel()
			if err != nil && !isUserNotFoundError(err) {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
				return
			}
		}
		next(w, r)
	}
}

func queryUserID(name string) userIDExtractor {
	return func(r *http.Request) string {
		return r.URL.Query().Get(name)
	}
}

func pathUserID() userIDExtractor {
	return func(r *http.Request) string {
		path := strings.TrimPrefix(r.URL.Path, "/v1/users/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 0 {
			return ""
		}
		return parts[0]
	}
}

func isUserNotFoundError(err error) bool {
	return errors.Is(err, store.ErrUserNotFound)
}
