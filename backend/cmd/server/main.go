package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/config"
	"github.com/Alumos/Clipmesh/backend/internal/events"
	"github.com/Alumos/Clipmesh/backend/internal/httpapi"
	"github.com/Alumos/Clipmesh/backend/internal/store"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.FilesDir, 0o755); err != nil {
		logger.Error("create files directory", "error", err)
		os.Exit(1)
	}
	clips, err := store.Open(cfg.DatabasePath, cfg.TextLimit)
	if err != nil {
		logger.Error("open clip store", "error", err)
		os.Exit(1)
	}
	defer clips.Close()
	if _, err := clips.EnsureAdmin(context.Background(), cfg.AdminUsername, cfg.AdminPassword); err != nil {
		logger.Error("ensure administrator", "error", err)
		os.Exit(1)
	}

	hub := events.NewHub()
	handler := httpapi.New(cfg, clips, hub, logger, version)
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		// SSE connections are intentionally long-lived; nginx and the
		// keep-alive heartbeat provide the relevant proxy-level boundaries.
		WriteTimeout: 0,
		IdleTimeout:  2 * time.Minute,
	}

	cleanupCtx, stopCleanup := context.WithCancel(context.Background())
	defer stopCleanup()
	go cleanupLoop(cleanupCtx, clips, hub, cfg.CleanupInterval, logger)

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("clipmesh api listening", "addr", cfg.Addr, "version", version)
		serverErrors <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped", "error", err)
			os.Exit(1)
		}
	case signal := <-signals:
		logger.Info("shutdown signal received", "signal", signal.String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown http server", "error", err)
		}
	}
}

func cleanupLoop(ctx context.Context, clips *store.Store, hub *events.Hub, interval time.Duration, logger *slog.Logger) {
	cleanup := func() {
		if err := clips.CleanupSessions(ctx); err != nil {
			logger.Error("cleanup sessions", "error", err)
		}
		expired, err := clips.CleanupExpired(ctx)
		if err != nil {
			logger.Error("cleanup expired clips", "error", err)
			return
		}
		for _, item := range expired {
			if item.StoragePath != "" {
				if err := os.Remove(item.StoragePath); err != nil && !errors.Is(err, os.ErrNotExist) {
					logger.Warn("remove expired file", "path", item.StoragePath, "error", err)
				}
			}
			hub.Publish(events.Event{Type: "deleted", UserID: item.UserID, ID: item.ID})
		}
		if len(expired) > 0 {
			logger.Info("expired files removed", "count", len(expired))
		}
	}

	cleanup()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}
