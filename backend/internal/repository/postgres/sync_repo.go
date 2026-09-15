package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncEventRepository defines database operations for sync playback events
type SyncEventRepository interface {
	Create(ctx context.Context, event *domain.SyncEvent) error
	CreateIfNoActive(ctx context.Context, event *domain.SyncEvent) error
	GetByID(ctx context.Context, id int) (*domain.SyncEvent, error)
	GetActive(ctx context.Context) (*domain.SyncEvent, error)
	MarkCompleted(ctx context.Context, id int) error
	MarkCancelled(ctx context.Context, id int) error
	RecoverActiveEvents(ctx context.Context) ([]domain.SyncEvent, error)
}

type syncRepo struct {
	pool *pgxpool.Pool
}

// NewSyncEventRepository creates a new PostgreSQL SyncEventRepository
func NewSyncEventRepository(pool *pgxpool.Pool) SyncEventRepository {
	return &syncRepo{pool: pool}
}

// Create inserts a new sync event record
func (r *syncRepo) Create(ctx context.Context, event *domain.SyncEvent) error {
	if event.Status == "" {
		event.Status = domain.SyncStatusActive
	}
	if event.TriggeredBy == "" {
		event.TriggeredBy = "admin"
	}

	query := `
		INSERT INTO sync_events (media_item_id, duration_seconds, started_at, ends_at, triggered_by, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		RETURNING id, created_at
	`
	err := r.pool.QueryRow(
		ctx,
		query,
		event.MediaItemID,
		event.DurationSeconds,
		event.StartedAt,
		event.EndsAt,
		event.TriggeredBy,
		event.Status,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert sync event: %w", err)
	}

	return nil
}

// CreateIfNoActive atomically verifies that no other unexpired active sync exists
// before inserting a new sync event. Uses a PostgreSQL transaction with an advisory
// lock to prevent race conditions across multiple instances or concurrent requests.
func (r *syncRepo) CreateIfNoActive(ctx context.Context, event *domain.SyncEvent) error {
	if event.Status == "" {
		event.Status = domain.SyncStatusActive
	}
	if event.TriggeredBy == "" {
		event.TriggeredBy = "operator"
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Acquire transaction-scoped advisory lock for sync operations (lock ID: 84848484)
	var acquired bool
	err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(84848484)").Scan(&acquired)
	if err != nil {
		return fmt.Errorf("failed to acquire sync advisory lock: %w", err)
	}
	if !acquired {
		return domain.NewSyncAlreadyActiveError()
	}

	// Check if any active sync exists whose ends_at is in the future
	var existingCount int
	checkQuery := `
		SELECT COUNT(*) FROM sync_events
		WHERE status = 'active' AND ends_at > $1
	`
	if err := tx.QueryRow(ctx, checkQuery, event.StartedAt).Scan(&existingCount); err != nil {
		return fmt.Errorf("failed to check active sync events: %w", err)
	}

	if existingCount > 0 {
		return domain.NewSyncAlreadyActiveError()
	}

	// Mark any older expired active events as 'completed'
	expireQuery := `
		UPDATE sync_events SET status = 'completed'
		WHERE status = 'active' AND ends_at <= $1
	`
	_, _ = tx.Exec(ctx, expireQuery, event.StartedAt)

	// Insert the new sync event
	insertQuery := `
		INSERT INTO sync_events (media_item_id, duration_seconds, started_at, ends_at, triggered_by, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		RETURNING id, created_at
	`
	err = tx.QueryRow(
		ctx,
		insertQuery,
		event.MediaItemID,
		event.DurationSeconds,
		event.StartedAt,
		event.EndsAt,
		event.TriggeredBy,
		event.Status,
	).Scan(&event.ID, &event.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert sync event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit sync event transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a sync event by ID joined with its media item details
func (r *syncRepo) GetByID(ctx context.Context, id int) (*domain.SyncEvent, error) {
	query := `
		SELECT
			s.id, s.media_item_id, s.duration_seconds, s.started_at, s.ends_at, s.triggered_by, s.status, s.created_at,
			m.id, m.title, m.type, m.url, m.default_duration_seconds, m.created_at, m.updated_at
		FROM sync_events s
		JOIN media_items m ON s.media_item_id = m.id
		WHERE s.id = $1
	`
	var s domain.SyncEvent
	var m domain.MediaItem

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&s.ID,
		&s.MediaItemID,
		&s.DurationSeconds,
		&s.StartedAt,
		&s.EndsAt,
		&s.TriggeredBy,
		&s.Status,
		&s.CreatedAt,
		&m.ID,
		&m.Title,
		&m.Type,
		&m.URL,
		&m.DefaultDurationSeconds,
		&m.CreatedAt,
		&m.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get sync event %d: %w", id, err)
	}

	s.Media = &m
	return &s, nil
}

// GetActive retrieves the currently active, unexpired sync event if one exists
func (r *syncRepo) GetActive(ctx context.Context) (*domain.SyncEvent, error) {
	query := `
		SELECT
			s.id, s.media_item_id, s.duration_seconds, s.started_at, s.ends_at, s.triggered_by, s.status, s.created_at,
			m.id, m.title, m.type, m.url, m.default_duration_seconds, m.created_at, m.updated_at
		FROM sync_events s
		JOIN media_items m ON s.media_item_id = m.id
		WHERE s.status = 'active' AND s.ends_at > CURRENT_TIMESTAMP
		ORDER BY s.id DESC
		LIMIT 1
	`
	var s domain.SyncEvent
	var m domain.MediaItem

	err := r.pool.QueryRow(ctx, query).Scan(
		&s.ID,
		&s.MediaItemID,
		&s.DurationSeconds,
		&s.StartedAt,
		&s.EndsAt,
		&s.TriggeredBy,
		&s.Status,
		&s.CreatedAt,
		&m.ID,
		&m.Title,
		&m.Type,
		&m.URL,
		&m.DefaultDurationSeconds,
		&m.CreatedAt,
		&m.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No active sync event currently
		}
		return nil, fmt.Errorf("failed to query active sync event: %w", err)
	}

	s.Media = &m
	return &s, nil
}

// MarkCompleted sets the event status to 'completed'
func (r *syncRepo) MarkCompleted(ctx context.Context, id int) error {
	query := `UPDATE sync_events SET status = 'completed' WHERE id = $1 AND status = 'active'`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark sync event %d completed: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkCancelled sets the event status to 'cancelled'
func (r *syncRepo) MarkCancelled(ctx context.Context, id int) error {
	query := `UPDATE sync_events SET status = 'cancelled' WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark sync event %d cancelled: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RecoverActiveEvents retrieves all active sync events whose ends_at is still in the future.
// Used by the server on boot to recover ongoing sync events after an unexpected restart.
func (r *syncRepo) RecoverActiveEvents(ctx context.Context) ([]domain.SyncEvent, error) {
	query := `
		SELECT
			s.id, s.media_item_id, s.duration_seconds, s.started_at, s.ends_at, s.triggered_by, s.status, s.created_at,
			m.id, m.title, m.type, m.url, m.default_duration_seconds, m.created_at, m.updated_at
		FROM sync_events s
		JOIN media_items m ON s.media_item_id = m.id
		WHERE s.status = 'active' AND s.ends_at > CURRENT_TIMESTAMP
		ORDER BY s.started_at ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query recoverable active sync events: %w", err)
	}
	defer rows.Close()

	var events []domain.SyncEvent
	for rows.Next() {
		var s domain.SyncEvent
		var m domain.MediaItem

		if err := rows.Scan(
			&s.ID,
			&s.MediaItemID,
			&s.DurationSeconds,
			&s.StartedAt,
			&s.EndsAt,
			&s.TriggeredBy,
			&s.Status,
			&s.CreatedAt,
			&m.ID,
			&m.Title,
			&m.Type,
			&m.URL,
			&m.DefaultDurationSeconds,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan sync event row: %w", err)
		}
		s.Media = &m
		events = append(events, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sync event rows: %w", err)
	}

	if events == nil {
		events = []domain.SyncEvent{}
	}

	return events, nil
}
