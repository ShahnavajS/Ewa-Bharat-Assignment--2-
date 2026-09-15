package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/eva-bharat/media-sequencer/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration represents a defined database schema migration
type Migration struct {
	Version  int64
	Name     string
	UpFile   string
	DownFile string
}

// AvailableMigrations lists all migrations in strict ascending order
var AvailableMigrations = []Migration{
	{
		Version:  1,
		Name:     "init_schema",
		UpFile:   "000001_init_schema.up.sql",
		DownFile: "000001_init_schema.down.sql",
	},
	{
		Version:  2,
		Name:     "seed_data",
		UpFile:   "000002_seed_data.up.sql",
		DownFile: "000002_seed_data.down.sql",
	},
}

// Migrator handles database schema migrations safely and idempotently
type Migrator struct {
	pool *pgxpool.Pool
}

// NewMigrator creates a new Migrator instance
func NewMigrator(pool *pgxpool.Pool) *Migrator {
	return &Migrator{pool: pool}
}

// EnsureSchemaMigrationsTable ensures the tracking table exists
func (m *Migrator) EnsureSchemaMigrationsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err := m.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}
	return nil
}

// IsApplied checks if a given migration version has already been executed
func (m *Migrator) IsApplied(ctx context.Context, version int64) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM schema_migrations WHERE version = $1`
	err := m.pool.QueryRow(ctx, query, version).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to query migration status for version %d: %w", version, err)
	}
	return count > 0, nil
}

// Up runs all unapplied migrations in ascending version order
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.EnsureSchemaMigrationsTable(ctx); err != nil {
		return err
	}

	for _, mig := range AvailableMigrations {
		applied, err := m.IsApplied(ctx, mig.Version)
		if err != nil {
			return err
		}

		if applied {
			slog.Debug("Migration already applied, skipping", "version", mig.Version, "name", mig.Name)
			continue
		}

		slog.Info("Applying migration", "version", mig.Version, "name", mig.Name)

		sqlBytes, err := migrations.FS.ReadFile(mig.UpFile)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", mig.UpFile, err)
		}

		tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", mig.Version, err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// Execute migration SQL statements
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", mig.UpFile, err)
		}

		// Record migration in tracking table
		recordQuery := `INSERT INTO schema_migrations (version, name, applied_at) VALUES ($1, $2, $3)`
		if _, err := tx.Exec(ctx, recordQuery, mig.Version, mig.Name, time.Now().UTC()); err != nil {
			return fmt.Errorf("failed to record migration %d in schema_migrations: %w", mig.Version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration transaction %d: %w", mig.Version, err)
		}

		slog.Info("Migration successfully applied", "version", mig.Version, "name", mig.Name)
	}

	return nil
}

// Down rolls back the most recent migration or all migrations if specified
func (m *Migrator) Down(ctx context.Context) error {
	if err := m.EnsureSchemaMigrationsTable(ctx); err != nil {
		return err
	}

	// Reverse order for rollback
	for i := len(AvailableMigrations) - 1; i >= 0; i-- {
		mig := AvailableMigrations[i]
		applied, err := m.IsApplied(ctx, mig.Version)
		if err != nil {
			return err
		}

		if !applied {
			continue
		}

		slog.Info("Rolling back migration", "version", mig.Version, "name", mig.Name)

		sqlBytes, err := migrations.FS.ReadFile(mig.DownFile)
		if err != nil {
			return fmt.Errorf("failed to read rollback file %s: %w", mig.DownFile, err)
		}

		tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("failed to begin transaction for rollback %d: %w", mig.Version, err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to execute rollback %s: %w", mig.DownFile, err)
		}

		deleteQuery := `DELETE FROM schema_migrations WHERE version = $1`
		if _, err := tx.Exec(ctx, deleteQuery, mig.Version); err != nil {
			return fmt.Errorf("failed to remove version %d from schema_migrations: %w", mig.Version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit rollback transaction %d: %w", mig.Version, err)
		}

		slog.Info("Migration rollback completed", "version", mig.Version, "name", mig.Name)
	}

	return nil
}
