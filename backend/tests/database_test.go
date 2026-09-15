package tests

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// getTestPool connects to the PostgreSQL database for testing
func getTestPool(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	testURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if testURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; integration tests require an isolated test database")
	}
	parsed, err := pgxpool.ParseConfig(testURL)
	if err != nil {
		t.Fatalf("Invalid TEST_DATABASE_URL: %v", err)
	}
	if !strings.Contains(strings.ToLower(parsed.ConnConfig.Database), "test") {
		t.Fatalf("Refusing to run destructive integration tests against non-test database %q", parsed.ConnConfig.Database)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, testURL)
	if err != nil {
		t.Skipf("Skipping PostgreSQL integration test: isolated database is unreachable: %v", err)
	}

	cleanup := func() {
		pool.Close()
	}

	return pool, cleanup
}

func TestMigrationLifecycleAndTables(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()

	ctx := context.Background()
	migrator := postgres.NewMigrator(pool)

	// 1. Run Migration Up
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up failed: %v", err)
	}

	// 2. Confirm all 4 core tables + tracking table exist
	expectedTables := []string{"windows", "media_items", "playlist_items", "sync_events", "schema_migrations"}
	for _, tableName := range expectedTables {
		var exists bool
		query := `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`
		if err := pool.QueryRow(ctx, query, tableName).Scan(&exists); err != nil {
			t.Fatalf("Failed to check table %s: %v", tableName, err)
		}
		if !exists {
			t.Errorf("Expected table %s to exist, but it was not found", tableName)
		}
	}

	// 3. Confirm Idempotency: Re-running Up does not fail or duplicate
	if err := migrator.Up(ctx); err != nil {
		t.Errorf("Migrator.Up re-run should be idempotent, but got: %v", err)
	}

	// 4. Confirm Indexes exist
	expectedIndexes := []string{"idx_playlist_window_pos", "idx_sync_events_active"}
	for _, indexName := range expectedIndexes {
		var exists bool
		query := `
			SELECT EXISTS (
				SELECT FROM pg_indexes
				WHERE schemaname = 'public' AND indexname = $1
			)
		`
		if err := pool.QueryRow(ctx, query, indexName).Scan(&exists); err != nil {
			t.Fatalf("Failed to check index %s: %v", indexName, err)
		}
		if !exists {
			t.Errorf("Expected index %s to exist, but it was not found", indexName)
		}
	}
}

func TestDatabaseConstraints(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()

	ctx := context.Background()
	migrator := postgres.NewMigrator(pool)
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up failed: %v", err)
	}

	// Clean test data before tests
	_, _ = pool.Exec(ctx, "DELETE FROM sync_events; DELETE FROM playlist_items; DELETE FROM media_items; DELETE FROM windows;")

	// Test 1: CHECK constraint - Invalid Media Type (should fail)
	_, err := pool.Exec(ctx, `
		INSERT INTO media_items (title, type, url, default_duration_seconds)
		VALUES ('Invalid Sound', 'audio', 'https://example.com/sound.mp3', 10)
	`)
	if err == nil {
		t.Errorf("Expected insert with invalid media type 'audio' to fail check constraint")
	} else if !strings.Contains(err.Error(), "check") && !strings.Contains(err.Error(), "violates") {
		t.Errorf("Expected check constraint error, got: %v", err)
	}

	// Test 2: CHECK constraint - Invalid Duration <= 0 (should fail)
	_, err = pool.Exec(ctx, `
		INSERT INTO media_items (title, type, url, default_duration_seconds)
		VALUES ('Zero Duration Media', 'image', 'https://example.com/img.png', 0)
	`)
	if err == nil {
		t.Errorf("Expected insert with duration 0 to fail check constraint")
	}

	// Test 3: Insert valid window & media item
	var windowID, mediaID1, mediaID2 int
	err = pool.QueryRow(ctx, `
		INSERT INTO windows (name, description, cycle_duration_seconds)
		VALUES ('Test Window 1', 'Test Description', 18000)
		RETURNING id
	`).Scan(&windowID)
	if err != nil {
		t.Fatalf("Failed to insert valid window: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO media_items (title, type, url, default_duration_seconds)
		VALUES ('Valid Image 1', 'image', 'https://example.com/valid1.png', 10)
		RETURNING id
	`).Scan(&mediaID1)
	if err != nil {
		t.Fatalf("Failed to insert valid media 1: %v", err)
	}

	err = pool.QueryRow(ctx, `
		INSERT INTO media_items (title, type, url, default_duration_seconds)
		VALUES ('Valid Image 2', 'image', 'https://example.com/valid2.png', 15)
		RETURNING id
	`).Scan(&mediaID2)
	if err != nil {
		t.Fatalf("Failed to insert valid media 2: %v", err)
	}

	// Test 4: Insert valid playlist item at position 1
	var playlistItemID int
	err = pool.QueryRow(ctx, `
		INSERT INTO playlist_items (window_id, media_item_id, position, duration_seconds)
		VALUES ($1, $2, 1, 10)
		RETURNING id
	`, windowID, mediaID1).Scan(&playlistItemID)
	if err != nil {
		t.Fatalf("Failed to insert valid playlist item: %v", err)
	}

	// Test 5: UNIQUE constraint - Duplicate position on same window (should fail)
	_, err = pool.Exec(ctx, `
		INSERT INTO playlist_items (window_id, media_item_id, position, duration_seconds)
		VALUES ($1, $2, 1, 15)
	`, windowID, mediaID2)
	if err == nil {
		t.Errorf("Expected insert with duplicate (window_id, position) to fail unique constraint")
	}

	// Test 6: CHECK constraint - Position < 1 (should fail)
	_, err = pool.Exec(ctx, `
		INSERT INTO playlist_items (window_id, media_item_id, position, duration_seconds)
		VALUES ($1, $2, 0, 15)
	`, windowID, mediaID2)
	if err == nil {
		t.Errorf("Expected insert with position 0 to fail check constraint")
	}

	// Test 7: Foreign key constraint - Nonexistent window (should fail)
	_, err = pool.Exec(ctx, `
		INSERT INTO playlist_items (window_id, media_item_id, position, duration_seconds)
		VALUES (99999, $1, 2, 15)
	`, mediaID2)
	if err == nil {
		t.Errorf("Expected insert referencing nonexistent window to fail foreign key")
	}

	// Test 8: Foreign key constraint - Nonexistent media (should fail)
	_, err = pool.Exec(ctx, `
		INSERT INTO playlist_items (window_id, media_item_id, position, duration_seconds)
		VALUES ($1, 99999, 2, 15)
	`, windowID)
	if err == nil {
		t.Errorf("Expected insert referencing nonexistent media to fail foreign key")
	}

	// Test 9: RESTRICT deletion on media item in active playlist (should fail)
	_, err = pool.Exec(ctx, `DELETE FROM media_items WHERE id = $1`, mediaID1)
	if err == nil {
		t.Errorf("Expected deleting media referenced by playlist item to be restricted")
	}

	// Test 10: SyncEvent CHECK constraint - Invalid status (should fail)
	_, err = pool.Exec(ctx, `
		INSERT INTO sync_events (media_item_id, duration_seconds, started_at, ends_at, status)
		VALUES ($1, 30, NOW(), NOW() + INTERVAL '30 seconds', 'unknown_status')
	`, mediaID1)
	if err == nil {
		t.Errorf("Expected insert with invalid sync status 'unknown_status' to fail check constraint")
	}

	// Test 11: CASCADE deletion on window deletes associated playlist items
	_, err = pool.Exec(ctx, `DELETE FROM windows WHERE id = $1`, windowID)
	if err != nil {
		t.Fatalf("Failed to delete window: %v", err)
	}

	var count int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM playlist_items WHERE window_id = $1`, windowID).Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 playlist items after cascading window deletion, found %d", count)
	}
}

func TestMigrationRollbackAndRecreate(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()

	ctx := context.Background()
	migrator := postgres.NewMigrator(pool)

	// Ensure Up first
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up failed: %v", err)
	}

	// Rollback
	if err := migrator.Down(ctx); err != nil {
		t.Fatalf("Migrator.Down failed: %v", err)
	}

	// Verify core tables dropped
	for _, tableName := range []string{"windows", "media_items", "playlist_items", "sync_events"} {
		var exists bool
		query := `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`
		_ = pool.QueryRow(ctx, query, tableName).Scan(&exists)
		if exists {
			t.Errorf("Expected table %s to be dropped after Down(), but it exists", tableName)
		}
	}

	// Reapply Up cleanly
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Re-applying Migrator.Up failed: %v", err)
	}
}
