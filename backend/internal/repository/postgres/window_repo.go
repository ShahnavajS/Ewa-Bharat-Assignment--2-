package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WindowRepository defines database operations for display windows
type WindowRepository interface {
	GetAll(ctx context.Context) ([]domain.Window, error)
	GetByID(ctx context.Context, id int) (*domain.Window, error)
	Create(ctx context.Context, window *domain.Window) error
	Update(ctx context.Context, window *domain.Window) error
	Delete(ctx context.Context, id int) error
}

type windowRepo struct {
	pool *pgxpool.Pool
}

// NewWindowRepository creates a new PostgreSQL WindowRepository
func NewWindowRepository(pool *pgxpool.Pool) WindowRepository {
	return &windowRepo{pool: pool}
}

// GetAll retrieves all windows ordered deterministically by ID
func (r *windowRepo) GetAll(ctx context.Context) ([]domain.Window, error) {
	query := `
		SELECT id, name, COALESCE(description, ''), cycle_duration_seconds, epoch_start_time, created_at, updated_at
		FROM windows
		ORDER BY id ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query windows: %w", err)
	}
	defer rows.Close()

	var windows []domain.Window
	for rows.Next() {
		var w domain.Window
		if err := rows.Scan(
			&w.ID,
			&w.Name,
			&w.Description,
			&w.CycleDurationSeconds,
			&w.EpochStartTime,
			&w.CreatedAt,
			&w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan window row: %w", err)
		}
		windows = append(windows, w)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating window rows: %w", err)
	}

	if windows == nil {
		windows = []domain.Window{}
	}

	return windows, nil
}

// GetByID retrieves a single window by its unique ID
func (r *windowRepo) GetByID(ctx context.Context, id int) (*domain.Window, error) {
	query := `
		SELECT id, name, COALESCE(description, ''), cycle_duration_seconds, epoch_start_time, created_at, updated_at
		FROM windows
		WHERE id = $1
	`
	var w domain.Window
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&w.ID,
		&w.Name,
		&w.Description,
		&w.CycleDurationSeconds,
		&w.EpochStartTime,
		&w.CreatedAt,
		&w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get window by id %d: %w", id, err)
	}

	return &w, nil
}

// Create inserts a new window into the database
func (r *windowRepo) Create(ctx context.Context, window *domain.Window) error {
	if window.CycleDurationSeconds <= 0 {
		window.CycleDurationSeconds = 18000 // 5-hour cycle default
	}
	if window.EpochStartTime.IsZero() {
		window.EpochStartTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	query := `
		INSERT INTO windows (name, description, cycle_duration_seconds, epoch_start_time, created_at, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at
	`
	err := r.pool.QueryRow(
		ctx,
		query,
		window.Name,
		window.Description,
		window.CycleDurationSeconds,
		window.EpochStartTime,
	).Scan(&window.ID, &window.CreatedAt, &window.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert window: %w", err)
	}

	return nil
}

// Update modifies an existing window's configuration
func (r *windowRepo) Update(ctx context.Context, window *domain.Window) error {
	query := `
		UPDATE windows
		SET name = $1, description = $2, cycle_duration_seconds = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING updated_at
	`
	err := r.pool.QueryRow(
		ctx,
		query,
		window.Name,
		window.Description,
		window.CycleDurationSeconds,
		window.ID,
	).Scan(&window.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to update window %d: %w", window.ID, err)
	}

	return nil
}

// Delete removes a window by ID (cascading its playlist items)
func (r *windowRepo) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM windows WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete window %d: %w", id, err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
