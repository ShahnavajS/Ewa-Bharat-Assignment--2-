package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/api"
	"github.com/eva-bharat/media-sequencer/internal/api/handlers"
	"github.com/eva-bharat/media-sequencer/internal/config"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
	"github.com/eva-bharat/media-sequencer/internal/service"
	"github.com/eva-bharat/media-sequencer/internal/ws"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Initialize structured logging handler
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting EVA Media Sequencer backend...")

	// 1. Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load application configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Configuration loaded successfully",
		"environment", cfg.Environment,
		"port", cfg.Port,
		"cors_origins", cfg.CORSAllowedOrigins,
	)

	// 2. Initialize PostgreSQL Connection Pool & Migrations
	var pool *pgxpool.Pool
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()

	pool, err = postgres.NewPool(dbCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Database connection failed", "error", err)
		os.Exit(1)
	} else {
		defer pool.Close()

		// Run pending database migrations
		migrator := postgres.NewMigrator(pool)
		if err := migrator.Up(dbCtx); err != nil {
			slog.Error("Failed to apply database migrations", "error", err)
			os.Exit(1)
		}
		slog.Info("Database schema verified and up to date")
	}

	// 3. Initialize Repositories
	windowRepo := postgres.NewWindowRepository(pool)
	mediaRepo := postgres.NewMediaRepository(pool)
	playlistRepo := postgres.NewPlaylistRepository(pool)
	syncRepo := postgres.NewSyncEventRepository(pool)

	// 4. Initialize Services
	windowSvc := service.NewWindowService(windowRepo)
	mediaSvc := service.NewMediaService(mediaRepo)
	playlistSvc := service.NewPlaylistService(playlistRepo, windowRepo, mediaRepo)

	syncSvc := service.NewSyncService(syncRepo, mediaRepo, nil)

	playbackEngine := service.NewPlaybackEngine()
	playbackSvc := service.NewPlaybackService(windowRepo, playlistRepo, playbackEngine, syncSvc)

	wsHub := ws.NewHub(playbackSvc, cfg.CORSAllowedOrigins...)
	go wsHub.Run()
	slog.Info("WebSocket hub started")

	// Wire WebSocket notifier into sync service
	syncSvc.SetNotifier(wsHub)

	// Crash recovery: reconcile any ongoing sync events on boot
	if err := syncSvc.Recover(context.Background()); err != nil {
		slog.Warn("Failed to recover active sync events on boot", "error", err)
	}

	// 5. Initialize HTTP Handlers
	healthHandler := handlers.NewHealthHandler(cfg, pool)
	windowHandler := handlers.NewWindowHandler(windowSvc, playbackSvc)
	mediaHandler := handlers.NewMediaHandler(mediaSvc)
	playlistHandler := handlers.NewPlaylistHandler(playlistSvc, wsHub)
	syncHandler := handlers.NewSyncHandler(syncSvc)

	// 6. Wire Router Dependencies
	deps := &api.RouterDeps{
		Config:          cfg,
		HealthHandler:   healthHandler,
		WindowHandler:   windowHandler,
		MediaHandler:    mediaHandler,
		PlaylistHandler: playlistHandler,
		WSHub:           wsHub,
		SyncHandler:     syncHandler,
	}

	router := api.SetupRouter(deps)

	// 7. Configure HTTP Server
	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:              serverAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 8. Start HTTP server in a separate goroutine
	go func() {
		slog.Info("HTTP server listening", "address", serverAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// 9. Graceful Shutdown listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	slog.Info("Shutdown signal received, initiating graceful shutdown...", "signal", sig.String())

	slog.Info("Shutting down WebSocket hub...")
	wsHub.Shutdown()
	slog.Info("WebSocket hub stopped")

	// Give active HTTP requests 10 seconds to finish
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown abruptly", "error", err)
	} else {
		slog.Info("Server stopped cleanly. Goodbye.")
	}
}
