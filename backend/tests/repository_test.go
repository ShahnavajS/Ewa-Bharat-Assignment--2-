package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func cleanupDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, _ = pool.Exec(ctx, "DELETE FROM sync_events; DELETE FROM playlist_items; DELETE FROM media_items; DELETE FROM windows;")
}

func TestWindowRepository(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()
	ctx := context.Background()

	migrator := postgres.NewMigrator(pool)
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up failed: %v", err)
	}
	cleanupDB(t, pool)

	repo := postgres.NewWindowRepository(pool)

	// 1. Create
	win := &domain.Window{
		Name:                 "Lobby Main",
		Description:          "Main display window in foyer",
		CycleDurationSeconds: 18000,
	}
	if err := repo.Create(ctx, win); err != nil {
		t.Fatalf("WindowRepo.Create failed: %v", err)
	}
	if win.ID <= 0 {
		t.Errorf("Expected positive window ID, got %d", win.ID)
	}

	// 2. GetByID
	fetched, err := repo.GetByID(ctx, win.ID)
	if err != nil {
		t.Fatalf("WindowRepo.GetByID failed: %v", err)
	}
	if fetched.Name != "Lobby Main" || fetched.CycleDurationSeconds != 18000 {
		t.Errorf("Fetched window mismatch: %+v", fetched)
	}

	// 3. GetAll
	all, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("WindowRepo.GetAll failed: %v", err)
	}
	if len(all) != 1 || all[0].ID != win.ID {
		t.Errorf("Expected 1 window, found %d", len(all))
	}

	// 4. Update
	fetched.Name = "Lobby Display Updated"
	fetched.Description = "Updated description"
	if err := repo.Update(ctx, fetched); err != nil {
		t.Fatalf("WindowRepo.Update failed: %v", err)
	}

	updated, _ := repo.GetByID(ctx, win.ID)
	if updated.Name != "Lobby Display Updated" {
		t.Errorf("Expected updated name 'Lobby Display Updated', got '%s'", updated.Name)
	}

	// 5. Delete
	if err := repo.Delete(ctx, win.ID); err != nil {
		t.Fatalf("WindowRepo.Delete failed: %v", err)
	}

	// 6. Verify ErrNotFound
	_, err = repo.GetByID(ctx, win.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Expected ErrNotFound after deletion, got: %v", err)
	}
}

func TestMediaRepository(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()
	ctx := context.Background()

	migrator := postgres.NewMigrator(pool)
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up failed: %v", err)
	}
	cleanupDB(t, pool)

	repo := postgres.NewMediaRepository(pool)

	// 1. Create Media Item (Image)
	img := &domain.MediaItem{
		Title:                  "Product Hero",
		Type:                   domain.MediaTypeImage,
		URL:                    "https://cdn.example.com/hero.jpg",
		DefaultDurationSeconds: 12,
	}
	if err := repo.Create(ctx, img); err != nil {
		t.Fatalf("MediaRepo.Create failed: %v", err)
	}

	// 2. Create Media Item (Blank)
	blank := &domain.MediaItem{
		Title:                  "Intermission Blank",
		Type:                   domain.MediaTypeBlank,
		URL:                    "blank",
		DefaultDurationSeconds: 5,
	}
	if err := repo.Create(ctx, blank); err != nil {
		t.Fatalf("MediaRepo.Create blank failed: %v", err)
	}

	// 3. GetByID
	fetched, err := repo.GetByID(ctx, img.ID)
	if err != nil {
		t.Fatalf("MediaRepo.GetByID failed: %v", err)
	}
	if fetched.Title != "Product Hero" || fetched.Type != domain.MediaTypeImage {
		t.Errorf("Fetched media mismatch: %+v", fetched)
	}

	// 4. GetAll
	all, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("MediaRepo.GetAll failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("Expected 2 media items, found %d", len(all))
	}

	// 5. Update
	fetched.Title = "Product Hero v2"
	fetched.DefaultDurationSeconds = 15
	if err := repo.Update(ctx, fetched); err != nil {
		t.Fatalf("MediaRepo.Update failed: %v", err)
	}

	// 6. Delete
	if err := repo.Delete(ctx, blank.ID); err != nil {
		t.Fatalf("MediaRepo.Delete failed: %v", err)
	}

	_, err = repo.GetByID(ctx, blank.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Expected ErrNotFound after deletion, got: %v", err)
	}
}

func TestPlaylistRepositoryOperationsAndCompaction(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()
	ctx := context.Background()

	migrator := postgres.NewMigrator(pool)
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up failed: %v", err)
	}
	cleanupDB(t, pool)

	winRepo := postgres.NewWindowRepository(pool)
	mediaRepo := postgres.NewMediaRepository(pool)
	playlistRepo := postgres.NewPlaylistRepository(pool)

	// Create window
	win := &domain.Window{Name: "Terminal 1", CycleDurationSeconds: 18000}
	_ = winRepo.Create(ctx, win)

	// Create 3 media items
	m1 := &domain.MediaItem{Title: "Ad 1", Type: domain.MediaTypeImage, URL: "url1", DefaultDurationSeconds: 10}
	m2 := &domain.MediaItem{Title: "Ad 2", Type: domain.MediaTypeVideo, URL: "url2", DefaultDurationSeconds: 20}
	m3 := &domain.MediaItem{Title: "Ad 3", Type: domain.MediaTypeImage, URL: "url3", DefaultDurationSeconds: 15}
	_ = mediaRepo.Create(ctx, m1)
	_ = mediaRepo.Create(ctx, m2)
	_ = mediaRepo.Create(ctx, m3)

	// 1. Add item 1
	p1, err := playlistRepo.Add(ctx, win.ID, m1.ID, 10)
	if err != nil {
		t.Fatalf("PlaylistRepo.Add p1 failed: %v", err)
	}
	if p1.Position != 1 {
		t.Errorf("Expected p1 position 1, got %d", p1.Position)
	}
	if p1.Media == nil || p1.Media.Title != "Ad 1" {
		t.Errorf("Expected joined media on Add, got: %+v", p1.Media)
	}

	// 2. Add item 2 (auto-assigned position 2)
	p2, err := playlistRepo.Add(ctx, win.ID, m2.ID, 20)
	if err != nil {
		t.Fatalf("PlaylistRepo.Add p2 failed: %v", err)
	}
	if p2.Position != 2 {
		t.Errorf("Expected p2 position 2, got %d", p2.Position)
	}

	// 3. Add item 3 (auto-assigned position 3)
	p3, err := playlistRepo.Add(ctx, win.ID, m3.ID, 15)
	if err != nil {
		t.Fatalf("PlaylistRepo.Add p3 failed: %v", err)
	}
	if p3.Position != 3 {
		t.Errorf("Expected p3 position 3, got %d", p3.Position)
	}

	// 4. Verify GetByWindowID returns ordered list
	items, err := playlistRepo.GetByWindowID(ctx, win.ID)
	if err != nil {
		t.Fatalf("GetByWindowID failed: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("Expected 3 playlist items, found %d", len(items))
	}
	if items[0].Position != 1 || items[1].Position != 2 || items[2].Position != 3 {
		t.Errorf("Unexpected ordering: %d, %d, %d", items[0].Position, items[1].Position, items[2].Position)
	}

	// 5. Test RESTRICT on media delete (cannot delete m1 while in playlist)
	err = mediaRepo.Delete(ctx, m1.ID)
	if !errors.Is(err, domain.ErrConstraintViolation) {
		t.Errorf("Expected ErrConstraintViolation when deleting referenced media, got: %v", err)
	}

	// 6. Test Safe Reorder: [p3, p1, p2] -> positions 1, 2, 3
	reorderPlan := []postgres.ReorderItem{
		{ID: p3.ID, Position: 1},
		{ID: p1.ID, Position: 2},
		{ID: p2.ID, Position: 3},
	}
	if err := playlistRepo.Reorder(ctx, win.ID, reorderPlan); err != nil {
		t.Fatalf("PlaylistRepo.Reorder failed: %v", err)
	}

	itemsReordered, _ := playlistRepo.GetByWindowID(ctx, win.ID)
	if itemsReordered[0].ID != p3.ID || itemsReordered[1].ID != p1.ID || itemsReordered[2].ID != p2.ID {
		t.Errorf("Reordering failed, got order: [%d, %d, %d]",
			itemsReordered[0].ID, itemsReordered[1].ID, itemsReordered[2].ID)
	}

	// 7. Test Delete & Auto-Compaction: Delete the middle item (p1 at pos 2)
	// Remaining items (p3 at pos 1, p2 at pos 3) must become pos 1 and pos 2!
	if err := playlistRepo.Delete(ctx, win.ID, p1.ID); err != nil {
		t.Fatalf("PlaylistRepo.Delete failed: %v", err)
	}

	itemsCompacted, _ := playlistRepo.GetByWindowID(ctx, win.ID)
	if len(itemsCompacted) != 2 {
		t.Fatalf("Expected 2 items after delete, got %d", len(itemsCompacted))
	}
	if itemsCompacted[0].ID != p3.ID || itemsCompacted[0].Position != 1 {
		t.Errorf("Expected p3 at position 1, got %+v", itemsCompacted[0])
	}
	if itemsCompacted[1].ID != p2.ID || itemsCompacted[1].Position != 2 {
		t.Errorf("Expected p2 compacted to position 2, got %+v", itemsCompacted[1])
	}
}

func TestConcurrentPlaylistAppends(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()
	ctx := context.Background()

	migrator := postgres.NewMigrator(pool)
	_ = migrator.Up(ctx)
	cleanupDB(t, pool)

	winRepo := postgres.NewWindowRepository(pool)
	mediaRepo := postgres.NewMediaRepository(pool)
	playlistRepo := postgres.NewPlaylistRepository(pool)

	win := &domain.Window{Name: "Concurrent Test Window", CycleDurationSeconds: 18000}
	_ = winRepo.Create(ctx, win)

	media := &domain.MediaItem{Title: "Ad Concurrent", Type: domain.MediaTypeImage, URL: "url", DefaultDurationSeconds: 10}
	_ = mediaRepo.Create(ctx, media)

	// Concurrently append 8 items from 8 goroutines
	numGoroutines := 8
	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := playlistRepo.Add(ctx, win.ID, media.ID, 10)
			if err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("Concurrent playlist Add failed: %v", err)
	}

	// Verify all 8 items exist with strictly unique positions 1..8
	items, err := playlistRepo.GetByWindowID(ctx, win.ID)
	if err != nil {
		t.Fatalf("Failed to fetch items: %v", err)
	}
	if len(items) != numGoroutines {
		t.Fatalf("Expected %d items, found %d", numGoroutines, len(items))
	}

	for i, item := range items {
		expectedPos := i + 1
		if item.Position != expectedPos {
			t.Errorf("Expected item %d to have position %d, got %d", i, expectedPos, item.Position)
		}
	}
}

func TestSyncEventRepository(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()
	ctx := context.Background()

	migrator := postgres.NewMigrator(pool)
	_ = migrator.Up(ctx)
	cleanupDB(t, pool)

	mediaRepo := postgres.NewMediaRepository(pool)
	syncRepo := postgres.NewSyncEventRepository(pool)

	media := &domain.MediaItem{Title: "Sync Emergency Alert", Type: domain.MediaTypeVideo, URL: "url_alert", DefaultDurationSeconds: 30}
	_ = mediaRepo.Create(ctx, media)

	// 1. Create Sync Event
	now := time.Now().UTC()
	endsAt := now.Add(30 * time.Second)
	event := &domain.SyncEvent{
		MediaItemID:     media.ID,
		DurationSeconds: 30,
		StartedAt:       now,
		EndsAt:          endsAt,
		TriggeredBy:     "operator1",
		Status:          domain.SyncStatusActive,
	}
	if err := syncRepo.Create(ctx, event); err != nil {
		t.Fatalf("SyncRepo.Create failed: %v", err)
	}
	if event.ID <= 0 {
		t.Errorf("Expected positive event ID, got %d", event.ID)
	}

	// 2. GetByID
	fetched, err := syncRepo.GetByID(ctx, event.ID)
	if err != nil {
		t.Fatalf("SyncRepo.GetByID failed: %v", err)
	}
	if fetched.Media == nil || fetched.Media.Title != "Sync Emergency Alert" {
		t.Errorf("Expected joined media on sync event: %+v", fetched.Media)
	}

	// 3. GetActive
	active, err := syncRepo.GetActive(ctx)
	if err != nil {
		t.Fatalf("SyncRepo.GetActive failed: %v", err)
	}
	if active == nil || active.ID != event.ID {
		t.Errorf("Expected active event %d, got %+v", event.ID, active)
	}

	// 4. RecoverActiveEvents
	recoverable, err := syncRepo.RecoverActiveEvents(ctx)
	if err != nil {
		t.Fatalf("SyncRepo.RecoverActiveEvents failed: %v", err)
	}
	if len(recoverable) != 1 || recoverable[0].ID != event.ID {
		t.Errorf("Expected 1 recoverable active event, got %d", len(recoverable))
	}

	// 5. MarkCompleted
	if err := syncRepo.MarkCompleted(ctx, event.ID); err != nil {
		t.Fatalf("SyncRepo.MarkCompleted failed: %v", err)
	}

	// 6. Verify GetActive returns nil once marked completed
	activeAfterComplete, err := syncRepo.GetActive(ctx)
	if err != nil {
		t.Fatalf("SyncRepo.GetActive after completion failed: %v", err)
	}
	if activeAfterComplete != nil {
		t.Errorf("Expected nil active sync after completion, got %+v", activeAfterComplete)
	}

	// 7. MarkCancelled on new event
	event2 := &domain.SyncEvent{
		MediaItemID:     media.ID,
		DurationSeconds: 15,
		StartedAt:       now,
		EndsAt:          now.Add(15 * time.Second),
		TriggeredBy:     "operator2",
		Status:          domain.SyncStatusActive,
	}
	_ = syncRepo.Create(ctx, event2)
	if err := syncRepo.MarkCancelled(ctx, event2.ID); err != nil {
		t.Fatalf("SyncRepo.MarkCancelled failed: %v", err)
	}
	cancelledEvent, _ := syncRepo.GetByID(ctx, event2.ID)
	if cancelledEvent.Status != domain.SyncStatusCancelled {
		t.Errorf("Expected status cancelled, got %s", cancelledEvent.Status)
	}
}
