package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"queue_up/backend/internal/config"
	server "queue_up/backend/internal/http"
	"queue_up/backend/internal/store"
)

func main() {
	cfg := config.Load()
	if strings.TrimSpace(cfg.SubmissionSanitizerWebhookURL) == "" {
		log.Fatal("SUBMISSION_SANITIZER_WEBHOOK_URL is required")
	}

	db, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	db.ConfigureSubmissionSanitizerWebhook(cfg.SubmissionSanitizerWebhookURL, cfg.SubmissionSanitizerWebhookTimeout)
	db.ConfigureLeetCodeAPI(cfg.LeetCodeAPIBaseURL, cfg.LeetCodeAPITimeout)

	h := server.New(db)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("backend api listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
