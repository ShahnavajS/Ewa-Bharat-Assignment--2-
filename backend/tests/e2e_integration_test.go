package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/api/handlers"
	"github.com/eva-bharat/media-sequencer/internal/config"
	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
	"github.com/eva-bharat/media-sequencer/internal/service"
	"github.com/eva-bharat/media-sequencer/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ─────────────────────────────────────────────────────────────────────────────
// 1. SEED DATA & COMPLETE PLAYBACK / SYNC LIFECYCLE
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_SeedAndPlaybackLifecycle(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()
	ctx := context.Background()

	// 1. Apply all migrations including 000002_seed_data
	migrator := postgres.NewMigrator(pool)
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up failed: %v", err)
	}

	// 2. Test Idempotency: re-running migrator.Up must succeed without errors
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Second Migrator.Up failed (idempotency violation): %v", err)
	}

	// 3. Verify Seeded Windows
	windowRepo := postgres.NewWindowRepository(pool)
	mediaRepo := postgres.NewMediaRepository(pool)
	playlistRepo := postgres.NewPlaylistRepository(pool)
	syncRepo := postgres.NewSyncEventRepository(pool)

	windows, err := windowRepo.GetAll(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch windows: %v", err)
	}
	if len(windows) < 3 {
		t.Fatalf("Expected at least 3 seeded windows, got %d", len(windows))
	}

	// Verify Window 1, 2, 3 names and cycle duration
	w1, err := windowRepo.GetByID(ctx, 1)
	if err != nil || w1.Name != "Lobby Display" || w1.CycleDurationSeconds != 18000 {
		t.Fatalf("Window 1 mismatch: %+v, err: %v", w1, err)
	}
	w2, err := windowRepo.GetByID(ctx, 2)
	if err != nil || w2.Name != "Reception Display" || w2.CycleDurationSeconds != 18000 {
		t.Fatalf("Window 2 mismatch: %+v, err: %v", w2, err)
	}
	w3, err := windowRepo.GetByID(ctx, 3)
	if err != nil || w3.Name != "Conference Display" || w3.CycleDurationSeconds != 18000 {
		t.Fatalf("Window 3 mismatch: %+v, err: %v", w3, err)
	}

	// 4. Verify Seeded Media Items (Images, Videos, Blank)
	mediaItems, err := mediaRepo.GetAll(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch media items: %v", err)
	}
	if len(mediaItems) < 8 {
		t.Fatalf("Expected at least 8 seeded media items, got %d", len(mediaItems))
	}

	blankMedia, err := mediaRepo.GetByID(ctx, 8)
	if err != nil || blankMedia.Type != domain.MediaTypeBlank {
		t.Fatalf("Expected Media ID 8 to be blank type, got %+v", blankMedia)
	}

	// 5. Verify Playlists
	pl1, err := playlistRepo.GetByWindowID(ctx, 1)
	if err != nil || len(pl1) < 3 {
		t.Fatalf("Window 1 playlist items mismatch: len=%d, err=%v", len(pl1), err)
	}
	pl2, err := playlistRepo.GetByWindowID(ctx, 2)
	if err != nil || len(pl2) < 2 {
		t.Fatalf("Window 2 playlist items mismatch: len=%d, err=%v", len(pl2), err)
	}
	pl3, err := playlistRepo.GetByWindowID(ctx, 3)
	if err != nil || len(pl3) < 3 {
		t.Fatalf("Window 3 playlist items mismatch: len=%d, err=%v", len(pl3), err)
	}
	// Verify Window 3 contains explicit blank item
	hasBlank := false
	for _, item := range pl3 {
		if item.Media != nil && item.Media.Type == domain.MediaTypeBlank {
			hasBlank = true
			break
		}
	}
	if !hasBlank {
		t.Errorf("Window 3 conference playlist should include configured blank media item")
	}

	// 6. Initialize Services
	playlistSvc := service.NewPlaylistService(playlistRepo, windowRepo, mediaRepo)
	syncSvc := service.NewSyncService(syncRepo, mediaRepo, nil)
	engine := service.NewPlaybackEngine()
	playbackSvc := service.NewPlaybackService(windowRepo, playlistRepo, engine, syncSvc)

	// Clean up any stale active sync before testing
	_, _ = pool.Exec(ctx, "UPDATE sync_events SET status = 'completed' WHERE status = 'active'")

	// 7. Test NORMAL Playback
	w1Resp, err := playbackSvc.GetWindowWithPlayback(ctx, 1)
	if err != nil {
		t.Fatalf("Window 1 playback resolution failed: %v", err)
	}
	if w1Resp.CurrentPlaybackState.Mode != domain.PlaybackModeNormal {
		t.Fatalf("Expected normal playback mode, got %q", w1Resp.CurrentPlaybackState.Mode)
	}
	if !w1Resp.CurrentPlaybackState.Active {
		t.Fatalf("Expected active playback state for Window 1")
	}

	w2Resp, err := playbackSvc.GetWindowWithPlayback(ctx, 2)
	if err != nil {
		t.Fatalf("Window 2 playback resolution failed: %v", err)
	}
	if w2Resp.CurrentPlaybackState.Mode != domain.PlaybackModeNormal {
		t.Fatalf("Expected normal playback mode for Window 2, got %q", w2Resp.CurrentPlaybackState.Mode)
	}

	// 8. Test Dynamic Playlist Update
	addedItem, err := playlistSvc.AddItem(ctx, 1, &service.AddPlaylistItemRequest{
		MediaID:         7, // Product Showcase Image
		DurationSeconds: 15,
	})
	if err != nil {
		t.Fatalf("Failed to add playlist item: %v", err)
	}
	if addedItem.Position != len(pl1)+1 {
		t.Fatalf("Expected new item position %d, got %d", len(pl1)+1, addedItem.Position)
	}

	// 9. Start Global Sync Override
	syncReq := &service.StartSyncRequest{
		MediaID:         7, // Product Showcase
		DurationSeconds: 30,
		TriggeredBy:     "integration-test",
	}
	syncEvent, err := syncSvc.Start(ctx, syncReq)
	if err != nil {
		t.Fatalf("Failed to start sync event: %v", err)
	}
	defer func() {
		_ = syncRepo.MarkCompleted(ctx, syncEvent.ID)
	}()

	// Verify both Window 1 and Window 2 report mode = "sync"
	w1Sync, err := playbackSvc.GetWindowWithPlayback(ctx, 1)
	if err != nil || w1Sync.CurrentPlaybackState.Mode != domain.PlaybackModeSync {
		t.Fatalf("Expected Window 1 mode sync, got %q, err: %v", w1Sync.CurrentPlaybackState.Mode, err)
	}
	if w1Sync.CurrentPlaybackState.Media == nil || w1Sync.CurrentPlaybackState.Media.ID != 7 {
		t.Fatalf("Expected sync media 7, got %+v", w1Sync.CurrentPlaybackState.Media)
	}

	w2Sync, err := playbackSvc.GetWindowWithPlayback(ctx, 2)
	if err != nil || w2Sync.CurrentPlaybackState.Mode != domain.PlaybackModeSync {
		t.Fatalf("Expected Window 2 mode sync, got %q, err: %v", w2Sync.CurrentPlaybackState.Mode, err)
	}
	if w2Sync.CurrentPlaybackState.Media == nil || w2Sync.CurrentPlaybackState.Media.ID != 7 {
		t.Fatalf("Expected sync media 7, got %+v", w2Sync.CurrentPlaybackState.Media)
	}

	// 10. Modify playlist during sync: sync remains dominant
	_, err = playlistSvc.AddItem(ctx, 2, &service.AddPlaylistItemRequest{
		MediaID:         3,
		DurationSeconds: 20,
	})
	if err != nil {
		t.Fatalf("Failed to modify playlist during sync: %v", err)
	}

	w2StillSync, err := playbackSvc.GetWindowWithPlayback(ctx, 2)
	if err != nil || w2StillSync.CurrentPlaybackState.Mode != domain.PlaybackModeSync {
		t.Fatalf("Expected sync to remain active after playlist modification, got %q", w2StillSync.CurrentPlaybackState.Mode)
	}

	// 11. End Sync and Verify Wall-Clock Resumption
	cancelled, err := syncSvc.Cancel(ctx)
	if err != nil {
		t.Fatalf("Failed to cancel sync event: %v", err)
	}
	if cancelled.Status != domain.SyncStatusCancelled {
		t.Fatalf("Expected cancelled status, got %q", cancelled.Status)
	}

	w1Normal, err := playbackSvc.GetWindowWithPlayback(ctx, 1)
	if err != nil || w1Normal.CurrentPlaybackState.Mode != domain.PlaybackModeNormal {
		t.Fatalf("Expected Window 1 to return to normal mode, got %q", w1Normal.CurrentPlaybackState.Mode)
	}

	w2Normal, err := playbackSvc.GetWindowWithPlayback(ctx, 2)
	if err != nil || w2Normal.CurrentPlaybackState.Mode != domain.PlaybackModeNormal {
		t.Fatalf("Expected Window 2 to return to normal mode, got %q", w2Normal.CurrentPlaybackState.Mode)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 2. RESTART RECOVERY INTEGRATION TEST
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_RestartRecovery(t *testing.T) {
	pool, cleanup := getTestPool(t)
	defer cleanup()
	ctx := context.Background()

	// Clean up any stale active sync
	_, _ = pool.Exec(ctx, "UPDATE sync_events SET status = 'completed' WHERE status = 'active'")

	mediaRepo := postgres.NewMediaRepository(pool)
	syncRepo := postgres.NewSyncEventRepository(pool)
	syncSvc := service.NewSyncService(syncRepo, mediaRepo, nil)

	// 1. Start a 60-second sync event
	event, err := syncSvc.Start(ctx, &service.StartSyncRequest{
		MediaID:         1,
		DurationSeconds: 60,
		TriggeredBy:     "recovery-test",
	})
	if err != nil {
		t.Fatalf("Failed to start sync event: %v", err)
	}
	defer func() {
		_ = syncRepo.MarkCompleted(ctx, event.ID)
	}()

	// 2. Simulate Backend Crash & Restart: instantiate a completely new service instance
	syncRepoRestarted := postgres.NewSyncEventRepository(pool)
	syncSvcRestarted := service.NewSyncService(syncRepoRestarted, mediaRepo, nil)

	// Recover should find the active sync event in the database
	if err := syncSvcRestarted.Recover(ctx); err != nil {
		t.Fatalf("SyncService.Recover failed on simulated reboot: %v", err)
	}

	active, err := syncSvcRestarted.GetActive(ctx)
	if err != nil || !active.Active {
		t.Fatalf("Expected active sync after simulated reboot, got: %+v, err: %v", active, err)
	}
	if active.SyncID != event.ID {
		t.Fatalf("Expected recovered sync ID %d, got %d", event.ID, active.SyncID)
	}

	// 3. Fast-forward time past expiration
	futureTime := event.EndsAt.Add(5 * time.Second)
	activeFuture, err := syncSvcRestarted.GetActiveAt(ctx, futureTime)
	if err != nil {
		t.Fatalf("GetActiveAt failed: %v", err)
	}
	if activeFuture.Active {
		t.Fatalf("Expected expired sync to report active=false")
	}

	// Give async DB update a moment
	time.Sleep(50 * time.Millisecond)

	// 4. Simulate a second restart after expiration
	syncSvcSecondReboot := service.NewSyncService(syncRepoRestarted, mediaRepo, nil)
	if err := syncSvcSecondReboot.Recover(ctx); err != nil {
		t.Fatalf("Second recover failed: %v", err)
	}

	activeAfterExpiry, err := syncSvcSecondReboot.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive failed: %v", err)
	}
	if activeAfterExpiry.Active {
		t.Fatalf("Expected expired sync to NOT be recovered or active on reboot")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 3. DETERMINISTIC PLAYBACK ENGINE BOUNDARY VERIFICATION
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_PlaybackEngineBoundaries(t *testing.T) {
	engine := service.NewPlaybackEngine()

	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	window := domain.Window{
		ID:                   10,
		Name:                 "Boundary Window",
		CycleDurationSeconds: 18000,
		EpochStartTime:       epoch,
	}

	media1 := &domain.MediaItem{ID: 1, Title: "Item 1", Type: domain.MediaTypeImage, DefaultDurationSeconds: 30}
	media2 := &domain.MediaItem{ID: 2, Title: "Item 2", Type: domain.MediaTypeVideo, DefaultDurationSeconds: 60}
	media3 := &domain.MediaItem{ID: 3, Title: "Item 3", Type: domain.MediaTypeImage, DefaultDurationSeconds: 20}

	playlist := []domain.PlaylistItem{
		{ID: 1, WindowID: 10, Position: 1, DurationSeconds: 30, IsActive: true, Media: media1},
		{ID: 2, WindowID: 10, Position: 2, DurationSeconds: 60, IsActive: true, Media: media2},
		{ID: 3, WindowID: 10, Position: 3, DurationSeconds: 20, IsActive: true, Media: media3},
	} // Total = 110s

	// Test 1: Exact item boundary at t = 0 (Item 1 start)
	s0, err := engine.Calculate(window, playlist, epoch)
	if err != nil || s0.Position != 1 || s0.ElapsedSeconds != 0 || s0.RemainingSeconds != 30 {
		t.Fatalf("Offset 0s mismatch: %+v, err: %v", s0, err)
	}

	// Test 2: Last second of Item 1 (offset 29s)
	s29, err := engine.Calculate(window, playlist, epoch.Add(29*time.Second))
	if err != nil || s29.Position != 1 || s29.ElapsedSeconds != 29 || s29.RemainingSeconds != 1 {
		t.Fatalf("Offset 29s mismatch: %+v", s29)
	}

	// Test 3: First second of Item 2 (offset 30s)
	s30, err := engine.Calculate(window, playlist, epoch.Add(30*time.Second))
	if err != nil || s30.Position != 2 || s30.ElapsedSeconds != 0 || s30.RemainingSeconds != 60 {
		t.Fatalf("Offset 30s boundary mismatch: %+v", s30)
	}

	// Test 4: Playlist shorter than cycle (110s loop repeats inside 18000s cycle)
	// Offset 110s should wrap back to Item 1 at elapsed 0s
	s110, err := engine.Calculate(window, playlist, epoch.Add(110*time.Second))
	if err != nil || s110.Position != 1 || s110.ElapsedSeconds != 0 {
		t.Fatalf("Offset 110s loop wrap mismatch: %+v", s110)
	}

	// Test 5: Exact cycle boundary at 18000s (Cycle offset 0s)
	sCycle1, err := engine.Calculate(window, playlist, epoch.Add(18000*time.Second))
	if err != nil || sCycle1.CyclePositionSeconds != 0 || sCycle1.Position != 1 {
		t.Fatalf("Cycle boundary 18000s mismatch: %+v", sCycle1)
	}

	// Test 6: Empty playlist
	sEmpty, err := engine.Calculate(window, nil, epoch.Add(50*time.Second))
	if err != nil || sEmpty.Active || !sEmpty.PlaylistEmpty {
		t.Fatalf("Empty playlist mismatch: %+v", sEmpty)
	}

	// Test 7: Future epoch
	sFuture, err := engine.Calculate(window, playlist, epoch.Add(-10*time.Minute))
	if err != nil || sFuture.Started || sFuture.Active {
		t.Fatalf("Future epoch mismatch: %+v", sFuture)
	}

	// Test 8: Determinism: same input + same timestamp -> exact same output
	calcTime := epoch.Add(4587 * time.Second)
	run1, _ := engine.Calculate(window, playlist, calcTime)
	run2, _ := engine.Calculate(window, playlist, calcTime)
	if run1.Position != run2.Position || run1.ElapsedSeconds != run2.ElapsedSeconds || run1.RemainingSeconds != run2.RemainingSeconds {
		t.Fatalf("Determinism failure: run1=%+v, run2=%+v", run1, run2)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 4. WEBSOCKET MULTI-CLIENT ISOLATION & GLOBAL OVERRIDE
// ─────────────────────────────────────────────────────────────────────────────

type fakeWSE2EPlaybackService struct {
	windows map[int]*service.WindowPlaybackResponse
}

func (f *fakeWSE2EPlaybackService) GetWindowWithPlayback(ctx context.Context, windowID int) (*service.WindowPlaybackResponse, error) {
	resp, ok := f.windows[windowID]
	if !ok {
		return nil, domain.NewWindowNotFoundError(windowID)
	}
	return resp, nil
}

func (f *fakeWSE2EPlaybackService) CalculateAt(ctx context.Context, windowID int, now time.Time) (*service.WindowPlaybackResponse, error) {
	return f.GetWindowWithPlayback(ctx, windowID)
}

func (f *fakeWSE2EPlaybackService) SetSyncService(syncSvc service.SyncService) {}

func TestE2E_WebSocketMultiClientIsolation(t *testing.T) {
	mockPlayback := &fakeWSE2EPlaybackService{
		windows: map[int]*service.WindowPlaybackResponse{
			1: {
				Window: domain.Window{ID: 1, Name: "Window 1", CycleDurationSeconds: 18000},
				Playlist: []domain.PlaylistItem{
					{ID: 101, WindowID: 1, Position: 1, DurationSeconds: 30, IsActive: true},
				},
				CurrentPlaybackState: domain.PlaybackState{Started: true, Active: true, Mode: domain.PlaybackModeNormal, WindowID: 1},
			},
			2: {
				Window: domain.Window{ID: 2, Name: "Window 2", CycleDurationSeconds: 18000},
				Playlist: []domain.PlaylistItem{
					{ID: 201, WindowID: 2, Position: 1, DurationSeconds: 45, IsActive: true},
				},
				CurrentPlaybackState: domain.PlaybackState{Started: true, Active: true, Mode: domain.PlaybackModeNormal, WindowID: 2},
			},
		},
	}

	hub := ws.NewHub(mockPlayback)
	go hub.Run()
	defer hub.Shutdown()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 1. Connect Client A and Client B
	connA, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Client A dial failed: %v", err)
	}
	defer connA.Close()

	connB, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Client B dial failed: %v", err)
	}
	defer connB.Close()

	// 2. Subscribe Client A to Window 1
	if err := connA.WriteJSON(map[string]any{"type": "SUBSCRIBE", "window_id": 1}); err != nil {
		t.Fatalf("Client A subscribe failed: %v", err)
	}

	// 3. Subscribe Client B to Window 2
	if err := connB.WriteJSON(map[string]any{"type": "SUBSCRIBE", "window_id": 2}); err != nil {
		t.Fatalf("Client B subscribe failed: %v", err)
	}

	// Read initial snapshots
	var msgA, msgB map[string]any
	_ = connA.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := connA.ReadJSON(&msgA); err != nil {
		t.Fatalf("Client A failed to receive initial snapshot: %v", err)
	}
	if msgA["type"] != "STATE_SNAPSHOT" {
		t.Fatalf("Expected Client A STATE_SNAPSHOT, got %v", msgA["type"])
	}

	_ = connB.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := connB.ReadJSON(&msgB); err != nil {
		t.Fatalf("Client B failed to receive initial snapshot: %v", err)
	}
	if msgB["type"] != "STATE_SNAPSHOT" {
		t.Fatalf("Expected Client B STATE_SNAPSHOT, got %v", msgB["type"])
	}

	// 3. Broadcast Window 1 Playlist Update (only Window 1 subscribers receive it)
	time.Sleep(50 * time.Millisecond) // ensure subscriptions are fully registered in hub
	hub.NotifyPlaylistUpdated(1, map[string]any{"updated": true})

	// Client A must receive PLAYLIST_UPDATED
	_ = connA.SetReadDeadline(time.Now().Add(1 * time.Second))
	var updateA map[string]any
	if err := connA.ReadJSON(&updateA); err != nil {
		t.Fatalf("Client A failed to receive PLAYLIST_UPDATED: %v", err)
	}
	if updateA["type"] != "PLAYLIST_UPDATED" {
		t.Fatalf("Expected Client A PLAYLIST_UPDATED, got %v", updateA["type"])
	}

	// Client B must NOT receive Window 1's playlist update: use 150ms read deadline
	// The underlying error (timeout) is expected; isolation verified by non-nil err
	_ = connB.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	_, _, leakErr := connB.ReadMessage()
	if leakErr == nil {
		t.Fatal("Client B isolation failure: received Window 1 playlist event when it should not")
	}

	// Reconnect Client B for global broadcast tests (deadline corruption workaround on Windows)
	connB.Close()
	connB2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Client B2 re-dial failed: %v", err)
	}
	defer connB2.Close()

	// Re-subscribe Client B2 to Window 2
	if err := connB2.WriteJSON(map[string]any{"type": "SUBSCRIBE", "window_id": 2}); err != nil {
		t.Fatalf("Client B2 re-subscribe failed: %v", err)
	}
	// Consume the STATE_SNAPSHOT
	_ = connB2.SetReadDeadline(time.Now().Add(2 * time.Second))
	var snap2 map[string]any
	if err := connB2.ReadJSON(&snap2); err != nil || snap2["type"] != "STATE_SNAPSHOT" {
		t.Fatalf("Client B2 snapshot failed: %v, got: %v", err, snap2["type"])
	}
	time.Sleep(30 * time.Millisecond) // ensure subscription registered

	// 4. Broadcast Global SYNC_STARTED -> BOTH clients must receive it
	hub.NotifySyncStarted(map[string]any{"sync_id": 99, "media_id": 5})

	_ = connA.SetReadDeadline(time.Now().Add(1 * time.Second))
	var syncA map[string]any
	if err := connA.ReadJSON(&syncA); err != nil || syncA["type"] != "SYNC_STARTED" {
		t.Fatalf("Client A failed to receive SYNC_STARTED: %v (msg=%v)", err, syncA)
	}

	_ = connB2.SetReadDeadline(time.Now().Add(1 * time.Second))
	var syncB map[string]any
	if err := connB2.ReadJSON(&syncB); err != nil || syncB["type"] != "SYNC_STARTED" {
		t.Fatalf("Client B2 failed to receive SYNC_STARTED: %v (msg=%v)", err, syncB)
	}

	// 5. Broadcast Global SYNC_ENDED -> BOTH clients must receive it
	hub.NotifySyncEnded(map[string]any{"sync_id": 99, "reason": "completed"})

	_ = connA.SetReadDeadline(time.Now().Add(1 * time.Second))
	var endA map[string]any
	if err := connA.ReadJSON(&endA); err != nil || endA["type"] != "SYNC_ENDED" {
		t.Fatalf("Client A failed to receive SYNC_ENDED: %v (msg=%v)", err, endA)
	}

	_ = connB2.SetReadDeadline(time.Now().Add(1 * time.Second))
	var endB map[string]any
	if err := connB2.ReadJSON(&endB); err != nil || endB["type"] != "SYNC_ENDED" {
		t.Fatalf("Client B2 failed to receive SYNC_ENDED: %v (msg=%v)", err, endB)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 5. HEALTH ENDPOINT DATABASE STATUS VERIFICATION
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_HealthEndpointDatabaseStatus(t *testing.T) {
	cfg := &config.Config{
		Port:        "8080",
		Environment: "test",
	}

	// Sub-test A: Unconfigured / nil pool -> 200 OK, database: "unconfigured"
	{
		h := handlers.NewHealthHandler(cfg, nil)
		rec := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
		c, _ := gin.CreateTestContext(rec)
		c.Request = req
		h.HealthCheck(c)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK for nil pool, got %d", rec.Code)
		}

		var envelope domain.ResponseEnvelope
		_ = json.Unmarshal(rec.Body.Bytes(), &envelope)
		if !envelope.Success {
			t.Fatalf("Expected success=true for nil pool")
		}
		data := envelope.Data.(map[string]any)
		if data["database"] != "unconfigured" {
			t.Fatalf("Expected database=unconfigured, got %v", data["database"])
		}
	}

	// Sub-test B: Live pool -> 200 OK, database: "connected"
	pool, cleanup := getTestPool(t)
	defer cleanup()

	{
		h := handlers.NewHealthHandler(cfg, pool)
		rec := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/health", nil)
		c, _ := gin.CreateTestContext(rec)
		c.Request = req
		h.HealthCheck(c)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK for live pool, got %d", rec.Code)
		}

		var envelope domain.ResponseEnvelope
		_ = json.Unmarshal(rec.Body.Bytes(), &envelope)
		if !envelope.Success {
			t.Fatalf("Expected success=true for live pool")
		}
		data := envelope.Data.(map[string]any)
		if data["database"] != "connected" {
			t.Fatalf("Expected database=connected, got %v", data["database"])
		}
	}
}
