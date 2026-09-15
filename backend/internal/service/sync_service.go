package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
)

// SyncEventNotifier is an interface enabling SyncService to broadcast
// global real-time synchronization events (WebSocket) without coupling
// directly to the ws package.
type SyncEventNotifier interface {
	NotifySyncStarted(syncData any)
	NotifySyncEnded(syncData any)
}

// StartSyncRequest represents the client payload to trigger a synchronized playback event.
type StartSyncRequest struct {
	MediaID         int    `json:"media_id"`
	DurationSeconds int    `json:"duration_seconds"`
	TriggeredBy     string `json:"triggered_by,omitempty"`
}

// SyncService coordinates synchronized media playback overrides across all display windows.
type SyncService interface {
	// Start initiates a global sync playback override for a specified duration.
	Start(ctx context.Context, req *StartSyncRequest) (*domain.SyncEvent, error)

	// GetActive returns the currently active sync state computed at time.Now() UTC.
	GetActive(ctx context.Context) (*domain.ActiveSyncState, error)

	// GetActiveAt computes active sync state at an explicit wall-clock instant.
	GetActiveAt(ctx context.Context, now time.Time) (*domain.ActiveSyncState, error)

	// Cancel prematurely terminates an ongoing sync event and restores normal playlists.
	Cancel(ctx context.Context) (*domain.SyncEvent, error)

	// Recover handles server crash recovery on boot, reconciling active sync events.
	Recover(ctx context.Context) error

	// SetNotifier dynamically wires the WebSocket event notifier.
	SetNotifier(notifier SyncEventNotifier)
}

type syncService struct {
	syncRepo  postgres.SyncEventRepository
	mediaRepo postgres.MediaRepository
	notifier  SyncEventNotifier
	ended     sync.Map
}

// NewSyncService creates a new SyncService with database repositories and an optional notifier.
func NewSyncService(
	syncRepo postgres.SyncEventRepository,
	mediaRepo postgres.MediaRepository,
	notifier SyncEventNotifier,
) SyncService {
	return &syncService{
		syncRepo:  syncRepo,
		mediaRepo: mediaRepo,
		notifier:  notifier,
	}
}

func (s *syncService) SetNotifier(notifier SyncEventNotifier) {
	s.notifier = notifier
}

// Start begins a global sync event with server-authoritative timestamps.
func (s *syncService) Start(ctx context.Context, req *StartSyncRequest) (*domain.SyncEvent, error) {
	// 1. Validate inputs
	if req.MediaID <= 0 {
		return nil, domain.NewValidationError("media_id must be a positive integer", nil)
	}
	if req.DurationSeconds <= 0 {
		return nil, domain.NewInvalidSyncDurationError("duration_seconds must be greater than 0")
	}
	if req.DurationSeconds > 18000 {
		return nil, domain.NewInvalidSyncDurationError("duration_seconds cannot exceed the 5-hour window cycle")
	}

	// 2. Verify media exists
	media, err := s.mediaRepo.GetByID(ctx, req.MediaID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewMediaNotFoundError(req.MediaID)
		}
		slog.Error("Failed to verify media item for sync", "error", err, "media_id", req.MediaID)
		return nil, domain.NewInternalServerError("Failed to verify media item")
	}

	// 3. Verify media type
	if media.Type != domain.MediaTypeImage && media.Type != domain.MediaTypeVideo && media.Type != domain.MediaTypeBlank {
		return nil, domain.NewInvalidMediaTypeError(fmt.Sprintf("Unsupported media type %q for sync playback", media.Type))
	}

	// 4. Set authoritative UTC timestamps
	startedAt := time.Now().UTC()
	endsAt := startedAt.Add(time.Duration(req.DurationSeconds) * time.Second)
	triggeredBy := req.TriggeredBy
	if triggeredBy == "" {
		triggeredBy = "operator"
	}

	event := &domain.SyncEvent{
		MediaItemID:     req.MediaID,
		DurationSeconds: req.DurationSeconds,
		StartedAt:       startedAt,
		EndsAt:          endsAt,
		TriggeredBy:     triggeredBy,
		Status:          domain.SyncStatusActive,
		Media:           media,
	}

	// 5. Atomic creation with active sync check
	if err := s.syncRepo.CreateIfNoActive(ctx, event); err != nil {
		return nil, err
	}

	slog.Info("sync event started",
		"id", event.ID,
		"media_id", event.MediaItemID,
		"duration_seconds", event.DurationSeconds,
		"started_at", event.StartedAt,
		"ends_at", event.EndsAt,
	)

	// 6. Broadcast global SYNC_STARTED event
	if s.notifier != nil {
		syncData := map[string]any{
			"event_id":         event.ID,
			"media":            media,
			"started_at":       event.StartedAt,
			"ends_at":          event.EndsAt,
			"duration_seconds": event.DurationSeconds,
		}
		s.notifier.NotifySyncStarted(syncData)
	}

	s.scheduleCompletion(event)

	return event, nil
}

// GetActive returns the active sync state at the current UTC time.
func (s *syncService) GetActive(ctx context.Context) (*domain.ActiveSyncState, error) {
	return s.GetActiveAt(ctx, time.Now().UTC())
}

// GetActiveAt computes active sync state at an explicit wall-clock instant.
func (s *syncService) GetActiveAt(ctx context.Context, now time.Time) (*domain.ActiveSyncState, error) {
	event, err := s.syncRepo.GetActive(ctx)
	if err != nil {
		slog.Error("Failed to query active sync", "error", err)
		return nil, domain.NewInternalServerError("Failed to query active sync event")
	}
	if event == nil {
		return &domain.ActiveSyncState{Active: false}, nil
	}

	// Wall-clock expiration check: if now >= ends_at, sync has expired
	if !now.Before(event.EndsAt) {
		s.complete(ctx, event.ID, "completed")

		return &domain.ActiveSyncState{Active: false}, nil
	}

	// Compute non-negative remaining duration
	remaining := event.EndsAt.Sub(now)
	remainingSeconds := int(remaining.Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}

	return &domain.ActiveSyncState{
		Active:           true,
		IsActive:         true,
		SyncID:           event.ID,
		Media:            event.Media,
		DurationSeconds:  event.DurationSeconds,
		StartedAt:        event.StartedAt,
		EndsAt:           event.EndsAt,
		RemainingSeconds: remainingSeconds,
		ServerTime:       now,
	}, nil
}

// Cancel terminates the current active sync event and broadcasts SYNC_ENDED.
func (s *syncService) Cancel(ctx context.Context) (*domain.SyncEvent, error) {
	event, err := s.syncRepo.GetActive(ctx)
	if err != nil {
		slog.Error("Failed to query active sync for cancellation", "error", err)
		return nil, domain.NewInternalServerError("Failed to query active sync event")
	}
	if event == nil || !time.Now().UTC().Before(event.EndsAt) {
		return nil, domain.NewSyncNotActiveError()
	}

	// Claim completion before updating storage so the expiry timer cannot emit a
	// second, conflicting SYNC_ENDED event while cancellation is in flight.
	s.ended.Store(event.ID, true)
	if err := s.syncRepo.MarkCancelled(ctx, event.ID); err != nil {
		s.ended.Delete(event.ID)
		slog.Error("Failed to mark sync event cancelled", "error", err, "event_id", event.ID)
		return nil, domain.NewInternalServerError("Failed to cancel sync event")
	}
	event.Status = domain.SyncStatusCancelled

	slog.Info("sync event cancelled", "id", event.ID)

	// Broadcast global SYNC_ENDED event
	if s.notifier != nil {
		s.notifier.NotifySyncEnded(map[string]any{
			"event_id": event.ID,
			"reason":   "cancelled",
		})
	}

	return event, nil
}

// Recover checks for running sync events upon server boot.
func (s *syncService) Recover(ctx context.Context) error {
	events, err := s.syncRepo.RecoverActiveEvents(ctx)
	if err != nil {
		return fmt.Errorf("failed to recover active sync events: %w", err)
	}

	now := time.Now().UTC()
	for _, event := range events {
		if !now.Before(event.EndsAt) {
			_ = s.syncRepo.MarkCompleted(ctx, event.ID)
			slog.Info("reconciled expired sync event on boot", "id", event.ID)
		} else {
			slog.Info("recovered ongoing active sync event on boot",
				"id", event.ID,
				"ends_at", event.EndsAt,
				"remaining_seconds", int(event.EndsAt.Sub(now).Seconds()),
			)
			s.scheduleCompletion(&event)
		}
	}

	return nil
}

func (s *syncService) scheduleCompletion(event *domain.SyncEvent) {
	delay := time.Until(event.EndsAt)
	if delay < 0 {
		delay = 0
	}
	time.AfterFunc(delay, func() {
		s.complete(context.Background(), event.ID, "completed")
	})
}

func (s *syncService) complete(ctx context.Context, eventID int, reason string) {
	if _, loaded := s.ended.LoadOrStore(eventID, true); loaded {
		return
	}
	if err := s.syncRepo.MarkCompleted(ctx, eventID); err != nil {
		s.ended.Delete(eventID)
		return
	}
	if s.notifier != nil {
		s.notifier.NotifySyncEnded(map[string]any{
			"event_id": eventID,
			"reason":   reason,
		})
	}
}
