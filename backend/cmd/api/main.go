package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raychua/factoryops/backend/internal/api"
	"github.com/raychua/factoryops/backend/internal/extraction"
	"github.com/raychua/factoryops/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	var data store.Store = store.NewMemory()
	var closeStore func()
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		postgres, err := store.NewPostgres(ctx, databaseURL)
		if err != nil { logger.Error("database startup failed", "error", err); os.Exit(1) }
		data, closeStore = postgres, postgres.Close
	}
	if closeStore != nil { defer closeStore() }

	extractor := extraction.New(os.Getenv("LLM_API_KEY"), os.Getenv("LLM_BASE_URL"), os.Getenv("LLM_MODEL"), nil)
	handler := api.New(data, extractor, logger).Handler()
	server := &http.Server{Addr: env("PORT", ":8080"), Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		logger.Info("FactoryOps API listening", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" { return value }
	return fallback
}
