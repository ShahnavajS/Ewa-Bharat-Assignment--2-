package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MediaRepository defines database operations for the global media asset library
type MediaRepository interface {
	GetAll(ctx context.Context) ([]domain.MediaItem, error)
	GetByID(ctx context.Context, id int) (*domain.MediaItem, error)
	Create(ctx context.Context, media *domain.MediaItem) error
	Update(ctx context.Context, media *domain.MediaItem) error
	Delete(ctx context.Context, id int) error
}

type mediaRepo struct {
	pool *pgxpool.Pool
}

// NewMediaRepository creates a new PostgreSQL MediaRepository
func NewMediaRepository(pool *pgxpool.Pool) MediaRepository {
	return &mediaRepo{pool: pool}
}

// GetAll returns all media items ordered deterministically by ID
func (r *mediaRepo) GetAll(ctx context.Context) ([]domain.MediaItem, error) {
	query := `
		SELECT id, title, type, url, default_duration_seconds, created_at, updated_at
		FROM media_items
		ORDER BY id ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query media items: %w", err)
	}
	defer rows.Close()

	var items []domain.MediaItem
	for rows.Next() {
		var m domain.MediaItem
		if err := rows.Scan(
			&m.ID,
			&m.Title,
			&m.Type,
			&m.URL,
			&m.DefaultDurationSeconds,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan media row: %w", err)
		}
		items = append(items, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating media rows: %w", err)
	}

	if items == nil {
		items = []domain.MediaItem{}
	}

	return items, nil
}

// GetByID retrieves a media item by its primary key
func (r *mediaRepo) GetByID(ctx context.Context, id int) (*domain.MediaItem, error) {
	query := `
		SELECT id, title, type, url, default_duration_seconds, created_at, updated_at
		FROM media_items
		WHERE id = $1
	`
	var m domain.MediaItem
	err := r.pool.QueryRow(ctx, query, id).Scan(
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
		return nil, fmt.Errorf("failed to get media item %d: %w", id, err)
	}

	return &m, nil
}

// Create inserts a new media asset into the library
func (r *mediaRepo) Create(ctx context.Context, media *domain.MediaItem) error {
	query := `
		INSERT INTO media_items (title, type, url, default_duration_seconds, created_at, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at
	`
	err := r.pool.QueryRow(
		ctx,
		query,
		media.Title,
		media.Type,
		media.URL,
		media.DefaultDurationSeconds,
	).Scan(&media.ID, &media.CreatedAt, &media.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert media item: %w", err)
	}

	return nil
}

// Update modifies an existing media asset
func (r *mediaRepo) Update(ctx context.Context, media *domain.MediaItem) error {
	query := `
		UPDATE media_items
		SET title = $1, type = $2, url = $3, default_duration_seconds = $4, updated_at = CURRENT_TIMESTAMP
		WHERE id = $5
		RETURNING updated_at
	`
	err := r.pool.QueryRow(
		ctx,
		query,
		media.Title,
		media.Type,
		media.URL,
		media.DefaultDurationSeconds,
		media.ID,
	).Scan(&media.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to update media item %d: %w", media.ID, err)
	}

	return nil
}

// Delete removes a media item by ID.
// If referenced by active playlist items or sync events, Postgres foreign key RESTRICT will error.
func (r *mediaRepo) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM media_items WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return domain.ErrConstraintViolation
		}
		return fmt.Errorf("failed to delete media item %d: %w", id, err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
