package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"sciplayer-api/internal/api"
	"sciplayer-api/internal/store/sqlite"
)

func main() {
	logger := log.New(os.Stdout, "sciplayer-api ", log.LstdFlags|log.LUTC)

	dbPath := envOrDefault("SCIPLAYER_DB_PATH", "data/sciplayer.db")
	addr := envOrDefault("SCIPLAYER_HTTP_ADDR", ":8090")

	store, err := sqlite.New(dbPath)
	if err != nil {
		logger.Fatalf("failed to initialize sqlite store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Printf("error closing store: %v", err)
		}
	}()

	handler := api.New(store, logger)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Printf("listening on %s", addr)

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
		logger.Fatalf("server stopped: %v", err)
	}
}

func envOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
