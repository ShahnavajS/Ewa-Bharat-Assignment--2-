package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReorderRequest defines an item ID and its desired 1-indexed sequence position
type ReorderItem struct {
	ID       int
	Position int
}

// PlaylistRepository defines operations for window playlist sequencing
type PlaylistRepository interface {
	GetByWindowID(ctx context.Context, windowID int) ([]domain.PlaylistItem, error)
	GetByID(ctx context.Context, id int) (*domain.PlaylistItem, error)
	Add(ctx context.Context, windowID, mediaID, durationSeconds int) (*domain.PlaylistItem, error)
	Delete(ctx context.Context, windowID, itemID int) error
	Reorder(ctx context.Context, windowID int, newOrder []ReorderItem) error
}

type playlistRepo struct {
	pool *pgxpool.Pool
}

// NewPlaylistRepository creates a new PostgreSQL PlaylistRepository
func NewPlaylistRepository(pool *pgxpool.Pool) PlaylistRepository {
	return &playlistRepo{pool: pool}
}

// GetByWindowID retrieves all playlist items for a window with joined MediaItem details,
// ordered deterministically by position ASC.
func (r *playlistRepo) GetByWindowID(ctx context.Context, windowID int) ([]domain.PlaylistItem, error) {
	query := `
		SELECT
			p.id, p.window_id, p.media_item_id, p.position, p.duration_seconds, p.is_active, p.created_at, p.updated_at,
			m.id, m.title, m.type, m.url, m.default_duration_seconds, m.created_at, m.updated_at
		FROM playlist_items p
		JOIN media_items m ON p.media_item_id = m.id
		WHERE p.window_id = $1 AND p.is_active = TRUE
		ORDER BY p.position ASC
	`
	rows, err := r.pool.Query(ctx, query, windowID)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlist items for window %d: %w", windowID, err)
	}
	defer rows.Close()

	var items []domain.PlaylistItem
	for rows.Next() {
		var p domain.PlaylistItem
		var m domain.MediaItem

		if err := rows.Scan(
			&p.ID,
			&p.WindowID,
			&p.MediaItemID,
			&p.Position,
			&p.DurationSeconds,
			&p.IsActive,
			&p.CreatedAt,
			&p.UpdatedAt,
			&m.ID,
			&m.Title,
			&m.Type,
			&m.URL,
			&m.DefaultDurationSeconds,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan playlist item row: %w", err)
		}

		p.Media = &m
		items = append(items, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating playlist item rows: %w", err)
	}

	if items == nil {
		items = []domain.PlaylistItem{}
	}

	return items, nil
}

// GetByID retrieves a single playlist item with its joined MediaItem
func (r *playlistRepo) GetByID(ctx context.Context, id int) (*domain.PlaylistItem, error) {
	query := `
		SELECT
			p.id, p.window_id, p.media_item_id, p.position, p.duration_seconds, p.is_active, p.created_at, p.updated_at,
			m.id, m.title, m.type, m.url, m.default_duration_seconds, m.created_at, m.updated_at
		FROM playlist_items p
		JOIN media_items m ON p.media_item_id = m.id
		WHERE p.id = $1
	`
	var p domain.PlaylistItem
	var m domain.MediaItem

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.WindowID,
		&p.MediaItemID,
		&p.Position,
		&p.DurationSeconds,
		&p.IsActive,
		&p.CreatedAt,
		&p.UpdatedAt,
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
		return nil, fmt.Errorf("failed to get playlist item %d: %w", id, err)
	}

	p.Media = &m
	return &p, nil
}

// Add atomically appends a new media item to the window's sequence.
// Concurrency protection: Locks the parent window row (SELECT ... FOR UPDATE) to prevent
// race conditions when computing the next position across concurrent requests.
func (r *playlistRepo) Add(ctx context.Context, windowID, mediaID, durationSeconds int) (*domain.PlaylistItem, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Lock the parent window row to serialize sequence appending for this window
	var cycleDuration int
	lockQuery := `SELECT cycle_duration_seconds FROM windows WHERE id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, lockQuery, windowID).Scan(&cycleDuration); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to lock window %d: %w", windowID, err)
	}

	// 2. Fetch media item default duration if not specified
	if durationSeconds <= 0 {
		mediaDurationQuery := `SELECT default_duration_seconds FROM media_items WHERE id = $1`
		if err := tx.QueryRow(ctx, mediaDurationQuery, mediaID).Scan(&durationSeconds); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, domain.ErrNotFound
			}
			return nil, fmt.Errorf("failed to get media item %d: %w", mediaID, err)
		}
	}

	// The configured sequence must fit inside the window's five-hour cycle.
	// The playback engine repeats shorter playlists; accepting a longer one would
	// make trailing items unreachable after the cycle resets.
	var existingDuration int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(duration_seconds), 0) FROM playlist_items WHERE window_id = $1 AND is_active = TRUE`,
		windowID,
	).Scan(&existingDuration); err != nil {
		return nil, fmt.Errorf("failed to calculate playlist duration: %w", err)
	}
	if existingDuration+durationSeconds > cycleDuration {
		return nil, domain.NewPlaylistTooLongError(existingDuration+durationSeconds, cycleDuration)
	}

	// 3. Compute next position safely
	var nextPos int
	posQuery := `SELECT COALESCE(MAX(position), 0) + 1 FROM playlist_items WHERE window_id = $1`
	if err := tx.QueryRow(ctx, posQuery, windowID).Scan(&nextPos); err != nil {
		return nil, fmt.Errorf("failed to determine next position: %w", err)
	}

	// 4. Insert playlist item
	var item domain.PlaylistItem
	insertQuery := `
		INSERT INTO playlist_items (window_id, media_item_id, position, duration_seconds, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, window_id, media_item_id, position, duration_seconds, is_active, created_at, updated_at
	`
	if err := tx.QueryRow(
		ctx,
		insertQuery,
		windowID,
		mediaID,
		nextPos,
		durationSeconds,
	).Scan(
		&item.ID,
		&item.WindowID,
		&item.MediaItemID,
		&item.Position,
		&item.DurationSeconds,
		&item.IsActive,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to insert playlist item: %w", err)
	}

	// 5. Fetch associated media info for immediate return
	var m domain.MediaItem
	mediaInfoQuery := `
		SELECT id, title, type, url, default_duration_seconds, created_at, updated_at
		FROM media_items
		WHERE id = $1
	`
	if err := tx.QueryRow(ctx, mediaInfoQuery, mediaID).Scan(
		&m.ID,
		&m.Title,
		&m.Type,
		&m.URL,
		&m.DefaultDurationSeconds,
		&m.CreatedAt,
		&m.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to fetch media info: %w", err)
	}
	item.Media = &m

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit add transaction: %w", err)
	}

	return &item, nil
}

// Delete removes a playlist item and automatically compacts remaining positions in a transaction.
// Positions remain dense (1, 2, 3...) without gaps.
func (r *playlistRepo) Delete(ctx context.Context, windowID, itemID int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to start delete transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Lock the parent window row
	var lockedWindowID int
	if err := tx.QueryRow(ctx, `SELECT id FROM windows WHERE id = $1 FOR UPDATE`, windowID).Scan(&lockedWindowID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to lock window: %w", err)
	}

	// 2. Find position of the item being deleted
	var deletedPos int
	findQuery := `SELECT position FROM playlist_items WHERE id = $1 AND window_id = $2`
	if err := tx.QueryRow(ctx, findQuery, itemID, windowID).Scan(&deletedPos); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to find playlist item: %w", err)
	}

	// 3. Delete the item
	deleteQuery := `DELETE FROM playlist_items WHERE id = $1`
	if _, err := tx.Exec(ctx, deleteQuery, itemID); err != nil {
		return fmt.Errorf("failed to delete playlist item: %w", err)
	}

	// 4. Compact remaining positions (shift down by 1)
	compactQuery := `
		UPDATE playlist_items
		SET position = position - 1, updated_at = CURRENT_TIMESTAMP
		WHERE window_id = $1 AND position > $2
	`
	if _, err := tx.Exec(ctx, compactQuery, windowID, deletedPos); err != nil {
		return fmt.Errorf("failed to compact playlist positions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit delete transaction: %w", err)
	}

	return nil
}

// Reorder applies a new deterministic position ordering across items in a window.
// Safe transactional 2-step approach:
//
//	Step 1: Set positions to temporary negative numbers to avoid unique constraint collisions.
//	Step 2: Set positions to their final positive values.
func (r *playlistRepo) Reorder(ctx context.Context, windowID int, newOrder []ReorderItem) error {
	if len(newOrder) == 0 {
		return nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to start reorder transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the parent window row
	var lockedWindowID int
	if err := tx.QueryRow(ctx, `SELECT id FROM windows WHERE id = $1 FOR UPDATE`, windowID).Scan(&lockedWindowID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to lock window: %w", err)
	}

	// Step 1: Temporarily set positions to large temporary values (>= 1,000,000)
	// to satisfy CHECK (position >= 1) while avoiding collisions with final positions (1..N)
	for i, item := range newOrder {
		tempPos := 1000000 + i + 1
		tempQuery := `UPDATE playlist_items SET position = $1 WHERE id = $2 AND window_id = $3`
		tag, err := tx.Exec(ctx, tempQuery, tempPos, item.ID, windowID)
		if err != nil {
			return fmt.Errorf("failed to set temporary position for item %d: %w", item.ID, err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
	}

	// Step 2: Set final positive positions
	for _, item := range newOrder {
		finalQuery := `
			UPDATE playlist_items
			SET position = $1, updated_at = CURRENT_TIMESTAMP
			WHERE id = $2 AND window_id = $3
		`
		if _, err := tx.Exec(ctx, finalQuery, item.Position, item.ID, windowID); err != nil {
			return fmt.Errorf("failed to set final position for item %d: %w", item.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit reorder transaction: %w", err)
	}

	return nil
}
