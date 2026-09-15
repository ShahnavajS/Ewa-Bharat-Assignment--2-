package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/api"
	"github.com/eva-bharat/media-sequencer/internal/api/handlers"
	"github.com/eva-bharat/media-sequencer/internal/config"
	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
	"github.com/eva-bharat/media-sequencer/internal/service"
	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test Fakes for API Testing
// ─────────────────────────────────────────────────────────────────────────────

type fakeSyncRepoAPI struct {
	mu     sync.Mutex
	events map[int]*domain.SyncEvent
	nextID int
}

func newFakeSyncRepoAPI() *fakeSyncRepoAPI {
	return &fakeSyncRepoAPI{
		events: make(map[int]*domain.SyncEvent),
		nextID: 1,
	}
}

func (r *fakeSyncRepoAPI) Create(ctx context.Context, event *domain.SyncEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	event.ID = r.nextID
	r.nextID++
	event.CreatedAt = time.Now().UTC()
	c := *event
	r.events[event.ID] = &c
	return nil
}

func (r *fakeSyncRepoAPI) CreateIfNoActive(ctx context.Context, event *domain.SyncEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, e := range r.events {
		if e.Status == domain.SyncStatusActive && e.EndsAt.After(event.StartedAt) {
			return domain.NewSyncAlreadyActiveError()
		}
	}

	event.ID = r.nextID
	r.nextID++
	event.CreatedAt = time.Now().UTC()
	c := *event
	r.events[event.ID] = &c
	return nil
}

func (r *fakeSyncRepoAPI) GetByID(ctx context.Context, id int) (*domain.SyncEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.events[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := *e
	return &c, nil
}

func (r *fakeSyncRepoAPI) GetActive(ctx context.Context) (*domain.SyncEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest *domain.SyncEvent
	for _, e := range r.events {
		if e.Status == domain.SyncStatusActive {
			if latest == nil || e.ID > latest.ID {
				c := *e
				latest = &c
			}
		}
	}
	return latest, nil
}

func (r *fakeSyncRepoAPI) MarkCompleted(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.events[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.SyncStatusCompleted
	return nil
}

func (r *fakeSyncRepoAPI) MarkCancelled(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.events[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.SyncStatusCancelled
	return nil
}

func (r *fakeSyncRepoAPI) RecoverActiveEvents(ctx context.Context) ([]domain.SyncEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var res []domain.SyncEvent
	for _, e := range r.events {
		if e.Status == domain.SyncStatusActive {
			res = append(res, *e)
		}
	}
	return res, nil
}

type fakeMediaRepoAPI struct {
	mu    sync.Mutex
	media map[int]*domain.MediaItem
}

func (r *fakeMediaRepoAPI) Create(ctx context.Context, item *domain.MediaItem) error { return nil }
func (r *fakeMediaRepoAPI) GetByID(ctx context.Context, id int) (*domain.MediaItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.media[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return m, nil
}
func (r *fakeMediaRepoAPI) GetAll(ctx context.Context) ([]domain.MediaItem, error)   { return nil, nil }
func (r *fakeMediaRepoAPI) Update(ctx context.Context, item *domain.MediaItem) error { return nil }
func (r *fakeMediaRepoAPI) Delete(ctx context.Context, id int) error                 { return nil }
func (r *fakeMediaRepoAPI) IsInUse(ctx context.Context, id int) (bool, error)        { return false, nil }

type fakeWindowRepoAPI struct {
	mu      sync.Mutex
	windows map[int]*domain.Window
}

func (r *fakeWindowRepoAPI) Create(ctx context.Context, w *domain.Window) error { return nil }
func (r *fakeWindowRepoAPI) GetByID(ctx context.Context, id int) (*domain.Window, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.windows[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return w, nil
}
func (r *fakeWindowRepoAPI) GetAll(ctx context.Context) ([]domain.Window, error) { return nil, nil }
func (r *fakeWindowRepoAPI) Update(ctx context.Context, w *domain.Window) error  { return nil }
func (r *fakeWindowRepoAPI) Delete(ctx context.Context, id int) error            { return nil }

type fakePlaylistRepoAPI struct {
	mu    sync.Mutex
	items map[int][]domain.PlaylistItem
}

func (r *fakePlaylistRepoAPI) GetByWindowID(ctx context.Context, windowID int) ([]domain.PlaylistItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.items[windowID], nil
}
func (r *fakePlaylistRepoAPI) GetByID(ctx context.Context, id int) (*domain.PlaylistItem, error) {
	return nil, nil
}
func (r *fakePlaylistRepoAPI) Add(ctx context.Context, windowID, mediaID, durationSeconds int) (*domain.PlaylistItem, error) {
	return nil, nil
}
func (r *fakePlaylistRepoAPI) Delete(ctx context.Context, windowID, itemID int) error { return nil }
func (r *fakePlaylistRepoAPI) Reorder(ctx context.Context, windowID int, newOrder []postgres.ReorderItem) error {
	return nil
}

func setupSyncAPIServer(t *testing.T) (*gin.Engine, *fakeSyncRepoAPI, *fakeMediaRepoAPI, service.SyncService, service.PlaybackService) {
	t.Helper()
	cfg := &config.Config{Environment: "test", Port: "8080"}

	syncRepo := newFakeSyncRepoAPI()
	mediaRepo := &fakeMediaRepoAPI{media: make(map[int]*domain.MediaItem)}
	windowRepo := &fakeWindowRepoAPI{windows: make(map[int]*domain.Window)}
	playlistRepo := &fakePlaylistRepoAPI{items: make(map[int][]domain.PlaylistItem)}

	// Seed Media
	mediaRepo.media[5] = &domain.MediaItem{
		ID:    5,
		Title: "Emergency Notice",
		Type:  domain.MediaTypeImage,
		URL:   "https://example.com/notice.jpg",
	}

	// Seed Window 1
	epoch := time.Now().UTC().Add(-10 * time.Minute)
	windowRepo.windows[1] = &domain.Window{
		ID:                   1,
		Name:                 "Window 1",
		CycleDurationSeconds: 18000,
		EpochStartTime:       epoch,
	}
	playlistRepo.items[1] = []domain.PlaylistItem{
		{ID: 101, WindowID: 1, MediaItemID: 5, Position: 1, DurationSeconds: 60, IsActive: true, Media: mediaRepo.media[5]},
	}

	syncSvc := service.NewSyncService(syncRepo, mediaRepo, nil)
	engine := service.NewPlaybackEngine()
	playbackSvc := service.NewPlaybackService(windowRepo, playlistRepo, engine, syncSvc)
	windowSvc := service.NewWindowService(windowRepo)

	syncHandler := handlers.NewSyncHandler(syncSvc)
	windowHandler := handlers.NewWindowHandler(windowSvc, playbackSvc)

	deps := &api.RouterDeps{
		Config:        cfg,
		HealthHandler: handlers.NewHealthHandler(cfg, nil),
		WindowHandler: windowHandler,
		SyncHandler:   syncHandler,
	}

	router := api.SetupRouter(deps)
	return router, syncRepo, mediaRepo, syncSvc, playbackSvc
}

// ─────────────────────────────────────────────────────────────────────────────
// REST API Tests for Sync Endpoints
// ─────────────────────────────────────────────────────────────────────────────

func TestREST_SyncEndpoints(t *testing.T) {
	router, _, _, syncSvc, _ := setupSyncAPIServer(t)

	// 1. GET /api/sync/state when inactive -> active: false
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/sync/state", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var stateResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &stateResp)
	if stateResp["active"] != false {
		t.Fatalf("expected active=false, got %v", stateResp["active"])
	}

	// 2. POST /api/sync with invalid media ID -> 404 NOT FOUND
	badMediaBody, _ := json.Marshal(map[string]any{"media_id": 999, "duration_seconds": 30})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/sync", bytes.NewReader(badMediaBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	// 3. POST /api/sync with invalid duration -> 400 BAD REQUEST
	badDurBody, _ := json.Marshal(map[string]any{"media_id": 5, "duration_seconds": 0})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/sync", bytes.NewReader(badDurBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// 4. POST /api/sync successfully -> 201 CREATED
	validBody, _ := json.Marshal(map[string]any{"media_id": 5, "duration_seconds": 30, "triggered_by": "operator"})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/sync", bytes.NewReader(validBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	// 5. POST /api/sync overlapping -> 409 CONFLICT
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/sync", bytes.NewReader(validBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict for overlapping sync, got %d", w.Code)
	}

	// 6. GET /api/sync/state when active -> active: true with details
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/sync/state", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var activeResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &activeResp)
	if activeResp["active"] != true {
		t.Fatalf("expected active=true, got %v", activeResp["active"])
	}
	if activeResp["media"] == nil {
		t.Fatal("expected media object in active state")
	}

	// 7. GET /api/windows/1 during sync -> effective state mode == "sync"
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/windows/1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var winResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &winResp)
	data := winResp["data"].(map[string]any)
	playbackState := data["current_playback_state"].(map[string]any)
	if playbackState["mode"] != "sync" {
		t.Fatalf("expected mode sync, got %v", playbackState["mode"])
	}

	// 8. POST /api/sync/cancel -> 200 OK
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/sync/cancel", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// 9. GET /api/windows/1 after cancellation -> effective state mode == "normal"
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/windows/1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &winResp)
	data = winResp["data"].(map[string]any)
	playbackState = data["current_playback_state"].(map[string]any)
	if playbackState["mode"] != "normal" {
		t.Fatalf("expected mode normal after cancellation, got %v", playbackState["mode"])
	}

	// 10. POST /api/sync/cancel when already cancelled -> 409 CONFLICT
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/sync/cancel", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 when cancelling inactive sync, got %d", w.Code)
	}

	_ = syncSvc
}
