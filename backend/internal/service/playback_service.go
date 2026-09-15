package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
)

// WindowPlaybackResponse is returned by GetWindowWithPlayback.
// It bundles the window configuration, its ordered playlist, and the
// effective current playback state computed at call time.
type WindowPlaybackResponse struct {
	domain.Window
	Playlist             []domain.PlaylistItem `json:"playlist"`
	CurrentPlaybackState domain.PlaybackState  `json:"current_playback_state"`
}

// PlaybackService retrieves a window + playlist from the repositories and
// computes the effective playback state (sync override or normal deterministic engine).
type PlaybackService interface {
	// GetWindowWithPlayback returns the window, its playlist, and its current
	// playback state computed at time.Now() UTC.
	GetWindowWithPlayback(ctx context.Context, windowID int) (*WindowPlaybackResponse, error)

	// CalculateAt lets callers pass an explicit `now` instead of using time.Now().
	CalculateAt(ctx context.Context, windowID int, now time.Time) (*WindowPlaybackResponse, error)

	// SetSyncService dynamically wires the SyncService to break circular initialization dependencies.
	SetSyncService(syncSvc SyncService)
}

type playbackService struct {
	windowRepo   postgres.WindowRepository
	playlistRepo postgres.PlaylistRepository
	engine       PlaybackEngine
	syncSvc      SyncService
}

// NewPlaybackService creates a new PlaybackService wired to repositories, engine, and optional SyncService.
func NewPlaybackService(
	windowRepo postgres.WindowRepository,
	playlistRepo postgres.PlaylistRepository,
	engine PlaybackEngine,
	syncSvc SyncService,
) PlaybackService {
	return &playbackService{
		windowRepo:   windowRepo,
		playlistRepo: playlistRepo,
		engine:       engine,
		syncSvc:      syncSvc,
	}
}

func (s *playbackService) SetSyncService(syncSvc SyncService) {
	s.syncSvc = syncSvc
}

func (s *playbackService) GetWindowWithPlayback(ctx context.Context, windowID int) (*WindowPlaybackResponse, error) {
	return s.CalculateAt(ctx, windowID, time.Now().UTC())
}

func (s *playbackService) CalculateAt(ctx context.Context, windowID int, now time.Time) (*WindowPlaybackResponse, error) {
	// 1. Fetch window
	window, err := s.windowRepo.GetByID(ctx, windowID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewWindowNotFoundError(windowID)
		}
		slog.Error("Failed to retrieve window for playback", "error", err, "window_id", windowID)
		return nil, domain.NewInternalServerError("Failed to retrieve window for playback")
	}

	// 2. Fetch playlist (ordered by position ASC, media joined)
	playlist, err := s.playlistRepo.GetByWindowID(ctx, windowID)
	if err != nil {
		slog.Error("Failed to retrieve playlist for playback", "error", err, "window_id", windowID)
		return nil, domain.NewInternalServerError("Failed to retrieve playlist for playback")
	}

	// 3. Check for active global sync override
	if s.syncSvc != nil {
		activeSync, err := s.syncSvc.GetActiveAt(ctx, now)
		if err == nil && activeSync != nil && activeSync.Active {
			elapsed := int(now.Sub(activeSync.StartedAt).Seconds())
			if elapsed < 0 {
				elapsed = 0
			}

			syncState := domain.PlaybackState{
				Mode:             domain.PlaybackModeSync,
				SyncEventID:      activeSync.SyncID,
				Started:          true,
				Active:           true,
				PlaylistEmpty:    false,
				WindowID:         windowID,
				Media:            activeSync.Media,
				ElapsedSeconds:   elapsed,
				RemainingSeconds: activeSync.RemainingSeconds,
				CalculatedAt:     now,
			}

			return &WindowPlaybackResponse{
				Window:               *window,
				Playlist:             playlist,
				CurrentPlaybackState: syncState,
			}, nil
		}
	}

	// 4. Delegate to pure engine for normal wall-clock playback
	state, err := s.engine.Calculate(*window, playlist, now)
	if err != nil {
		slog.Error("Playback engine error", "error", err, "window_id", window.ID)
		return nil, domain.NewInternalServerError("Playback engine calculation error")
	}

	// 5. Annotate mode
	if state.Active {
		state.Mode = domain.PlaybackModeNormal
	} else {
		state.Mode = domain.PlaybackModeInactive
	}

	return &WindowPlaybackResponse{
		Window:               *window,
		Playlist:             playlist,
		CurrentPlaybackState: state,
	}, nil
}
