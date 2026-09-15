package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/repository/postgres"
	"github.com/eva-bharat/media-sequencer/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test Fakes
// ─────────────────────────────────────────────────────────────────────────────

type fakeSyncRepo struct {
	mu     sync.Mutex
	events map[int]*domain.SyncEvent
	nextID int
}

func newFakeSyncRepo() *fakeSyncRepo {
	return &fakeSyncRepo{
		events: make(map[int]*domain.SyncEvent),
		nextID: 1,
	}
}

func (r *fakeSyncRepo) Create(ctx context.Context, event *domain.SyncEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	event.ID = r.nextID
	r.nextID++
	event.CreatedAt = time.Now().UTC()
	c := *event
	r.events[event.ID] = &c
	return nil
}

func (r *fakeSyncRepo) CreateIfNoActive(ctx context.Context, event *domain.SyncEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, e := range r.events {
		if e.Status == domain.SyncStatusActive && e.EndsAt.After(event.StartedAt) {
			return domain.NewSyncAlreadyActiveError()
		}
	}

	// Mark older active events as completed
	for _, e := range r.events {
		if e.Status == domain.SyncStatusActive && !e.EndsAt.After(event.StartedAt) {
			e.Status = domain.SyncStatusCompleted
		}
	}

	event.ID = r.nextID
	r.nextID++
	event.CreatedAt = time.Now().UTC()
	c := *event
	r.events[event.ID] = &c
	return nil
}

func (r *fakeSyncRepo) GetByID(ctx context.Context, id int) (*domain.SyncEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.events[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := *e
	return &c, nil
}

func (r *fakeSyncRepo) GetActive(ctx context.Context) (*domain.SyncEvent, error) {
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

func (r *fakeSyncRepo) MarkCompleted(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.events[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.SyncStatusCompleted
	return nil
}

func (r *fakeSyncRepo) MarkCancelled(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.events[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.Status = domain.SyncStatusCancelled
	return nil
}

func (r *fakeSyncRepo) RecoverActiveEvents(ctx context.Context) ([]domain.SyncEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []domain.SyncEvent
	for _, e := range r.events {
		if e.Status == domain.SyncStatusActive {
			result = append(result, *e)
		}
	}
	return result, nil
}

type fakeMediaRepo struct {
	mu    sync.Mutex
	media map[int]*domain.MediaItem
}

func newFakeMediaRepo() *fakeMediaRepo {
	return &fakeMediaRepo{
		media: make(map[int]*domain.MediaItem),
	}
}

func (r *fakeMediaRepo) Create(ctx context.Context, item *domain.MediaItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.media[item.ID] = item
	return nil
}

func (r *fakeMediaRepo) GetByID(ctx context.Context, id int) (*domain.MediaItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.media[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return item, nil
}

func (r *fakeMediaRepo) GetAll(ctx context.Context) ([]domain.MediaItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var res []domain.MediaItem
	for _, m := range r.media {
		res = append(res, *m)
	}
	return res, nil
}

func (r *fakeMediaRepo) Update(ctx context.Context, item *domain.MediaItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.media[item.ID] = item
	return nil
}

func (r *fakeMediaRepo) Delete(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.media, id)
	return nil
}

func (r *fakeMediaRepo) IsInUse(ctx context.Context, id int) (bool, error) {
	return false, nil
}

type fakeNotifier struct {
	mu           sync.Mutex
	startedCalls []any
	endedCalls   []any
}

func (n *fakeNotifier) NotifySyncStarted(data any) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.startedCalls = append(n.startedCalls, data)
}

func (n *fakeNotifier) NotifySyncEnded(data any) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.endedCalls = append(n.endedCalls, data)
}

type fakeWindowRepo struct {
	mu      sync.Mutex
	windows map[int]*domain.Window
}

func newFakeWindowRepo() *fakeWindowRepo {
	return &fakeWindowRepo{windows: make(map[int]*domain.Window)}
}

func (r *fakeWindowRepo) Create(ctx context.Context, w *domain.Window) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.windows[w.ID] = w
	return nil
}

func (r *fakeWindowRepo) GetByID(ctx context.Context, id int) (*domain.Window, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.windows[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return w, nil
}

func (r *fakeWindowRepo) GetAll(ctx context.Context) ([]domain.Window, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var list []domain.Window
	for _, w := range r.windows {
		list = append(list, *w)
	}
	return list, nil
}

func (r *fakeWindowRepo) Update(ctx context.Context, w *domain.Window) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.windows[w.ID] = w
	return nil
}

func (r *fakeWindowRepo) Delete(ctx context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.windows, id)
	return nil
}

type fakePlaylistRepo struct {
	mu    sync.Mutex
	items map[int][]domain.PlaylistItem
}

func newFakePlaylistRepo() *fakePlaylistRepo {
	return &fakePlaylistRepo{items: make(map[int][]domain.PlaylistItem)}
}

func (r *fakePlaylistRepo) GetByWindowID(ctx context.Context, windowID int) ([]domain.PlaylistItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.items[windowID], nil
}

func (r *fakePlaylistRepo) GetByID(ctx context.Context, id int) (*domain.PlaylistItem, error) {
	return nil, nil
}

func (r *fakePlaylistRepo) Add(ctx context.Context, windowID, mediaID, durationSeconds int) (*domain.PlaylistItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := &domain.PlaylistItem{
		ID:              len(r.items[windowID]) + 1,
		WindowID:        windowID,
		MediaItemID:     mediaID,
		DurationSeconds: durationSeconds,
		Position:        len(r.items[windowID]) + 1,
		IsActive:        true,
	}
	r.items[windowID] = append(r.items[windowID], *item)
	return item, nil
}

func (r *fakePlaylistRepo) Delete(ctx context.Context, windowID, itemID int) error {
	return nil
}

func (r *fakePlaylistRepo) Reorder(ctx context.Context, windowID int, newOrder []postgres.ReorderItem) error {
	return nil
}

func (r *fakePlaylistRepo) addItemDirect(item *domain.PlaylistItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.WindowID] = append(r.items[item.WindowID], *item)
}

// ─────────────────────────────────────────────────────────────────────────────
// SYNC SERVICE TESTS (Scenarios 1 - 11)
// ─────────────────────────────────────────────────────────────────────────────

func TestSyncService_StartSyncSuccessfully(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	notifier := &fakeNotifier{}
	svc := service.NewSyncService(syncRepo, mediaRepo, notifier)

	// Seed media
	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{
		ID:    5,
		Title: "Emergency Alert",
		Type:  domain.MediaTypeImage,
		URL:   "https://example.com/alert.png",
	})

	req := &service.StartSyncRequest{
		MediaID:         5,
		DurationSeconds: 30,
		TriggeredBy:     "operator",
	}

	event, err := svc.Start(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if event.ID == 0 {
		t.Fatal("expected assigned event ID")
	}
	if event.Status != domain.SyncStatusActive {
		t.Fatalf("expected status active, got %s", event.Status)
	}
	if event.DurationSeconds != 30 {
		t.Fatalf("expected duration 30, got %d", event.DurationSeconds)
	}

	// Verify WebSocket notification fired
	notifier.mu.Lock()
	if len(notifier.startedCalls) != 1 {
		t.Fatalf("expected 1 started notification, got %d", len(notifier.startedCalls))
	}
	notifier.mu.Unlock()
}

func TestSyncService_RejectMissingMedia(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	svc := service.NewSyncService(syncRepo, mediaRepo, nil)

	req := &service.StartSyncRequest{MediaID: 999, DurationSeconds: 30}
	_, err := svc.Start(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing media")
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeMediaNotFound {
		t.Fatalf("expected MEDIA_NOT_FOUND, got %v", err)
	}
}

func TestSyncService_RejectInvalidDuration(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	svc := service.NewSyncService(syncRepo, mediaRepo, nil)

	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{
		ID:   1,
		Type: domain.MediaTypeImage,
	})

	for _, d := range []int{0, -5, -100, 18001} {
		req := &service.StartSyncRequest{MediaID: 1, DurationSeconds: d}
		_, err := svc.Start(context.Background(), req)
		if err == nil {
			t.Fatalf("expected error for duration %d", d)
		}
		var appErr *domain.AppError
		if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeInvalidSyncDuration {
			t.Fatalf("expected INVALID_SYNC_DURATION for duration %d, got %v", d, err)
		}
	}
}

func TestSyncService_NaturalCompletionNotifiesClients(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	notifier := &fakeNotifier{}
	svc := service.NewSyncService(syncRepo, mediaRepo, notifier)

	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{ID: 1, Type: domain.MediaTypeImage})
	event, err := svc.Start(context.Background(), &service.StartSyncRequest{MediaID: 1, DurationSeconds: 1})
	if err != nil {
		t.Fatalf("start sync: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		notifier.mu.Lock()
		ended := len(notifier.endedCalls)
		notifier.mu.Unlock()
		if ended == 1 {
			stored, getErr := syncRepo.GetByID(context.Background(), event.ID)
			if getErr != nil || stored.Status != domain.SyncStatusCompleted {
				t.Fatalf("expected completed event, got %#v (%v)", stored, getErr)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("natural completion did not notify clients")
}

func TestSyncService_GetActiveAndNoActive(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	svc := service.NewSyncService(syncRepo, mediaRepo, nil)

	// When no sync has been triggered
	state, err := svc.GetActive(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Active {
		t.Fatal("expected active=false when no sync")
	}

	// Trigger sync
	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{ID: 1, Type: domain.MediaTypeImage})
	_, _ = svc.Start(context.Background(), &service.StartSyncRequest{MediaID: 1, DurationSeconds: 40})

	state, err = svc.GetActive(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !state.Active {
		t.Fatal("expected active=true after start")
	}
	if state.RemainingSeconds <= 0 || state.RemainingSeconds > 40 {
		t.Fatalf("expected remaining in (0, 40], got %d", state.RemainingSeconds)
	}
}

func TestSyncService_RejectOverlappingSync(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	svc := service.NewSyncService(syncRepo, mediaRepo, nil)

	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{ID: 1, Type: domain.MediaTypeImage})
	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{ID: 2, Type: domain.MediaTypeVideo})

	// 1. First sync starts
	_, err := svc.Start(context.Background(), &service.StartSyncRequest{MediaID: 1, DurationSeconds: 60})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// 2. Second overlapping sync must fail with SYNC_ALREADY_ACTIVE
	_, err = svc.Start(context.Background(), &service.StartSyncRequest{MediaID: 2, DurationSeconds: 30})
	if err == nil {
		t.Fatal("expected conflict error for overlapping sync")
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeSyncAlreadyActive {
		t.Fatalf("expected SYNC_ALREADY_ACTIVE, got %v", err)
	}
}

func TestSyncService_CancelActiveAndInactive(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	notifier := &fakeNotifier{}
	svc := service.NewSyncService(syncRepo, mediaRepo, notifier)

	// Cancel when no sync is active -> SYNC_NOT_ACTIVE
	_, err := svc.Cancel(context.Background())
	if err == nil {
		t.Fatal("expected error cancelling inactive sync")
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != domain.ErrCodeSyncNotActive {
		t.Fatalf("expected SYNC_NOT_ACTIVE, got %v", err)
	}

	// Start sync
	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{ID: 1, Type: domain.MediaTypeImage})
	_, _ = svc.Start(context.Background(), &service.StartSyncRequest{MediaID: 1, DurationSeconds: 50})

	// Cancel active sync
	cancelled, err := svc.Cancel(context.Background())
	if err != nil {
		t.Fatalf("failed to cancel active sync: %v", err)
	}
	if cancelled.Status != domain.SyncStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", cancelled.Status)
	}

	// Verify WebSocket notification fired
	notifier.mu.Lock()
	if len(notifier.endedCalls) != 1 {
		t.Fatalf("expected 1 ended notification, got %d", len(notifier.endedCalls))
	}
	notifier.mu.Unlock()

	// Subsequent GetActive should return active=false
	state, _ := svc.GetActive(context.Background())
	if state.Active {
		t.Fatal("expected active=false after cancellation")
	}
}

func TestSyncService_ExpiredSyncBecomesInactive(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	svc := service.NewSyncService(syncRepo, mediaRepo, nil)

	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{ID: 1, Type: domain.MediaTypeImage})

	// Start sync with 10s duration
	event, err := svc.Start(context.Background(), &service.StartSyncRequest{MediaID: 1, DurationSeconds: 10})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// 5 seconds into sync -> active
	now5 := event.StartedAt.Add(5 * time.Second)
	st5, err := svc.GetActiveAt(context.Background(), now5)
	if err != nil || !st5.Active || st5.RemainingSeconds != 5 {
		t.Fatalf("expected active with 5s remaining, got active=%v, rem=%d, err=%v", st5.Active, st5.RemainingSeconds, err)
	}

	// 10 seconds into sync (exact end) -> expired!
	now10 := event.EndsAt
	st10, err := svc.GetActiveAt(context.Background(), now10)
	if err != nil || st10.Active {
		t.Fatalf("expected inactive at exact ends_at, got active=%v", st10.Active)
	}

	// 15 seconds into sync (past end) -> expired!
	now15 := event.EndsAt.Add(5 * time.Second)
	st15, err := svc.GetActiveAt(context.Background(), now15)
	if err != nil || st15.Active {
		t.Fatalf("expected inactive after ends_at, got active=%v", st15.Active)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TIME BOUNDARY TESTS (Section 27)
// ─────────────────────────────────────────────────────────────────────────────

func TestSyncService_TimeBoundaries(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	svc := service.NewSyncService(syncRepo, mediaRepo, nil)

	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{ID: 1, Type: domain.MediaTypeImage})
	event, _ := svc.Start(context.Background(), &service.StartSyncRequest{MediaID: 1, DurationSeconds: 30})

	// 1. now = started_at → full duration remaining
	stStart, _ := svc.GetActiveAt(context.Background(), event.StartedAt)
	if !stStart.Active || stStart.RemainingSeconds != 30 {
		t.Fatalf("at started_at: expected active=true, remaining=30, got active=%v, rem=%d", stStart.Active, stStart.RemainingSeconds)
	}

	// 2. now = ends_at - 1ns → active
	stBeforeEnd, _ := svc.GetActiveAt(context.Background(), event.EndsAt.Add(-1*time.Nanosecond))
	if !stBeforeEnd.Active || stBeforeEnd.RemainingSeconds < 0 {
		t.Fatalf("at ends_at - 1ns: expected active=true, got %v", stBeforeEnd.Active)
	}

	// 3. now = ends_at → expired
	stAtEnd, _ := svc.GetActiveAt(context.Background(), event.EndsAt)
	if stAtEnd.Active {
		t.Fatal("at ends_at: expected active=false")
	}

	// 4. now = ends_at + 1ns → expired
	stAfterEnd, _ := svc.GetActiveAt(context.Background(), event.EndsAt.Add(1*time.Nanosecond))
	if stAfterEnd.Active {
		t.Fatal("at ends_at + 1ns: expected active=false")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CONCURRENCY TESTS (Scenarios 21 - 22)
// ─────────────────────────────────────────────────────────────────────────────

func TestSyncService_ConcurrentStartSync(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	svc := service.NewSyncService(syncRepo, mediaRepo, nil)

	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{ID: 1, Type: domain.MediaTypeImage})

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	successCount := 0
	conflictCount := 0
	var countMu sync.Mutex

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.Start(context.Background(), &service.StartSyncRequest{MediaID: 1, DurationSeconds: 20})
			countMu.Lock()
			defer countMu.Unlock()
			if err == nil {
				successCount++
			} else {
				var appErr *domain.AppError
				if errors.As(err, &appErr) && appErr.Code == domain.ErrCodeSyncAlreadyActive {
					conflictCount++
				}
			}
		}()
	}

	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful StartSync, got %d", successCount)
	}
	if conflictCount != goroutines-1 {
		t.Fatalf("expected %d conflict errors, got %d", goroutines-1, conflictCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CRASH RECOVERY TESTS (Scenarios 23 - 25)
// ─────────────────────────────────────────────────────────────────────────────

func TestSyncService_CrashRecovery(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	svc := service.NewSyncService(syncRepo, mediaRepo, nil)

	_ = mediaRepo.Create(context.Background(), &domain.MediaItem{ID: 1, Type: domain.MediaTypeImage})

	// 1. Simulate active sync event in DB created before crash
	now := time.Now().UTC()
	ongoingEvent := &domain.SyncEvent{
		MediaItemID:     1,
		DurationSeconds: 120,
		StartedAt:       now.Add(-30 * time.Second),
		EndsAt:          now.Add(90 * time.Second), // still active
		Status:          domain.SyncStatusActive,
	}
	_ = syncRepo.Create(context.Background(), ongoingEvent)

	// 2. Simulate expired event that was never marked completed due to crash
	expiredEvent := &domain.SyncEvent{
		MediaItemID:     1,
		DurationSeconds: 60,
		StartedAt:       now.Add(-120 * time.Second),
		EndsAt:          now.Add(-60 * time.Second), // expired
		Status:          domain.SyncStatusActive,
	}
	_ = syncRepo.Create(context.Background(), expiredEvent)

	// 3. Boot server and call Recover
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatalf("recover failed: %v", err)
	}

	// Verify ongoing event remains active
	active, err := svc.GetActive(context.Background())
	if err != nil || !active.Active || active.SyncID != ongoingEvent.ID {
		t.Fatalf("expected ongoing sync %d to be active, got %+v", ongoingEvent.ID, active)
	}

	// Verify expired event was marked completed
	reloadedExpired, _ := syncRepo.GetByID(context.Background(), expiredEvent.ID)
	if reloadedExpired.Status != domain.SyncStatusCompleted {
		t.Fatalf("expected expired event to be marked completed, got %s", reloadedExpired.Status)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// EFFECTIVE PLAYBACK TESTS (Scenarios 12 - 20)
// ─────────────────────────────────────────────────────────────────────────────

func TestEffectivePlayback_NormalAndSyncOverride(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	windowRepo := newFakeWindowRepo()
	playlistRepo := newFakePlaylistRepo()

	syncSvc := service.NewSyncService(syncRepo, mediaRepo, nil)
	engine := service.NewPlaybackEngine()
	playbackSvc := service.NewPlaybackService(windowRepo, playlistRepo, engine, syncSvc)

	// Setup media
	mediaA := &domain.MediaItem{ID: 1, Title: "Item A", Type: domain.MediaTypeVideo, DefaultDurationSeconds: 30}
	mediaB := &domain.MediaItem{ID: 2, Title: "Item B", Type: domain.MediaTypeVideo, DefaultDurationSeconds: 60}
	mediaSync := &domain.MediaItem{ID: 99, Title: "Global Sync Media", Type: domain.MediaTypeImage, DefaultDurationSeconds: 45}

	_ = mediaRepo.Create(context.Background(), mediaA)
	_ = mediaRepo.Create(context.Background(), mediaB)
	_ = mediaRepo.Create(context.Background(), mediaSync)

	// Setup window: Epoch at 10:00:00 UTC
	epoch := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	window := &domain.Window{
		ID:                   1,
		Name:                 "Window 1",
		CycleDurationSeconds: 18000,
		EpochStartTime:       epoch,
	}
	_ = windowRepo.Create(context.Background(), window)

	// Playlist: A (30s) + B (60s) = 90s total
	playlistRepo.addItemDirect(&domain.PlaylistItem{
		ID: 101, WindowID: 1, MediaItemID: 1, Position: 1, DurationSeconds: 30, IsActive: true, Media: mediaA,
	})
	playlistRepo.addItemDirect(&domain.PlaylistItem{
		ID: 102, WindowID: 1, MediaItemID: 2, Position: 2, DurationSeconds: 60, IsActive: true, Media: mediaB,
	})

	// 12. Normal Playback with No Sync (at 10:00:10 UTC -> 10s elapsed into Item A)
	t1 := epoch.Add(10 * time.Second)
	respNormal, err := playbackSvc.CalculateAt(context.Background(), 1, t1)
	if err != nil {
		t.Fatalf("playback calc failed: %v", err)
	}
	if respNormal.CurrentPlaybackState.Mode != domain.PlaybackModeNormal {
		t.Fatalf("expected mode normal, got %s", respNormal.CurrentPlaybackState.Mode)
	}
	if respNormal.CurrentPlaybackState.PlaylistItemID != 101 || respNormal.CurrentPlaybackState.ElapsedSeconds != 10 {
		t.Fatalf("unexpected state: %+v", respNormal.CurrentPlaybackState)
	}

	// 13-15. Start Sync: 10:00:20 to 10:01:00 (40s duration)
	syncEvent := &domain.SyncEvent{
		MediaItemID:     99,
		DurationSeconds: 40,
		StartedAt:       epoch.Add(20 * time.Second),
		EndsAt:          epoch.Add(60 * time.Second),
		Status:          domain.SyncStatusActive,
		Media:           mediaSync,
	}
	_ = syncRepo.Create(context.Background(), syncEvent)

	// Query at 10:00:30 (10s into sync, 30s remaining)
	tSync := epoch.Add(30 * time.Second)
	respSync, err := playbackSvc.CalculateAt(context.Background(), 1, tSync)
	if err != nil {
		t.Fatalf("sync playback calc failed: %v", err)
	}
	if respSync.CurrentPlaybackState.Mode != domain.PlaybackModeSync {
		t.Fatalf("expected mode sync, got %s", respSync.CurrentPlaybackState.Mode)
	}
	if respSync.CurrentPlaybackState.SyncEventID != syncEvent.ID {
		t.Fatalf("expected sync event ID %d, got %d", syncEvent.ID, respSync.CurrentPlaybackState.SyncEventID)
	}
	if respSync.CurrentPlaybackState.Media.ID != 99 {
		t.Fatalf("expected sync media ID 99, got %d", respSync.CurrentPlaybackState.Media.ID)
	}
	if respSync.CurrentPlaybackState.RemainingSeconds != 30 {
		t.Fatalf("expected remaining 30s, got %d", respSync.CurrentPlaybackState.RemainingSeconds)
	}
	// Underlyling playlist MUST be preserved
	if len(respSync.Playlist) != 2 {
		t.Fatalf("expected underlying playlist preserved, got %d items", len(respSync.Playlist))
	}

	// 16-17. Wall-clock Resume After Expiration: Query at 10:01:10 (70s into epoch)
	// Normal sequence: A is [0, 30), B is [30, 90).
	// At 70s, Item B MUST be playing with 40s elapsed and 20s remaining!
	tAfterSync := epoch.Add(70 * time.Second)
	respResumed, err := playbackSvc.CalculateAt(context.Background(), 1, tAfterSync)
	if err != nil {
		t.Fatalf("resumed playback calc failed: %v", err)
	}
	if respResumed.CurrentPlaybackState.Mode != domain.PlaybackModeNormal {
		t.Fatalf("expected mode normal after sync ended, got %s", respResumed.CurrentPlaybackState.Mode)
	}
	if respResumed.CurrentPlaybackState.PlaylistItemID != 102 {
		t.Fatalf("expected Item B (102) playing at 70s, got %d", respResumed.CurrentPlaybackState.PlaylistItemID)
	}
	if respResumed.CurrentPlaybackState.ElapsedSeconds != 40 {
		t.Fatalf("expected 40s elapsed into Item B, got %d", respResumed.CurrentPlaybackState.ElapsedSeconds)
	}
	if respResumed.CurrentPlaybackState.RemainingSeconds != 20 {
		t.Fatalf("expected 20s remaining in Item B, got %d", respResumed.CurrentPlaybackState.RemainingSeconds)
	}
}

func TestEffectivePlayback_PlaylistChangesDuringSyncReflectedAfterward(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	windowRepo := newFakeWindowRepo()
	playlistRepo := newFakePlaylistRepo()

	syncSvc := service.NewSyncService(syncRepo, mediaRepo, nil)
	engine := service.NewPlaybackEngine()
	playbackSvc := service.NewPlaybackService(windowRepo, playlistRepo, engine, syncSvc)

	epoch := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	window := &domain.Window{ID: 2, CycleDurationSeconds: 18000, EpochStartTime: epoch}
	_ = windowRepo.Create(context.Background(), window)

	mediaSync := &domain.MediaItem{ID: 99, Type: domain.MediaTypeImage}
	mediaNew := &domain.MediaItem{ID: 3, Type: domain.MediaTypeVideo}
	_ = mediaRepo.Create(context.Background(), mediaSync)
	_ = mediaRepo.Create(context.Background(), mediaNew)

	// Sync active from 10:00:00 to 10:01:00
	_ = syncRepo.Create(context.Background(), &domain.SyncEvent{
		MediaItemID:     99,
		DurationSeconds: 60,
		StartedAt:       epoch,
		EndsAt:          epoch.Add(60 * time.Second),
		Status:          domain.SyncStatusActive,
		Media:           mediaSync,
	})

	// During sync, add a new 100s playlist item
	playlistRepo.addItemDirect(&domain.PlaylistItem{
		ID: 201, WindowID: 2, MediaItemID: 3, Position: 1, DurationSeconds: 100, IsActive: true, Media: mediaNew,
	})

	// At 10:01:10 (10s after sync ends, 70s into epoch)
	// Normal playback should immediately evaluate the new playlist!
	tAfter := epoch.Add(70 * time.Second)
	resp, err := playbackSvc.CalculateAt(context.Background(), 2, tAfter)
	if err != nil {
		t.Fatalf("calculation failed: %v", err)
	}
	if resp.CurrentPlaybackState.Mode != domain.PlaybackModeNormal {
		t.Fatalf("expected mode normal, got %s", resp.CurrentPlaybackState.Mode)
	}
	if resp.CurrentPlaybackState.PlaylistItemID != 201 {
		t.Fatalf("expected new item 201 playing, got %d", resp.CurrentPlaybackState.PlaylistItemID)
	}
	if resp.CurrentPlaybackState.ElapsedSeconds != 70 {
		t.Fatalf("expected 70s elapsed, got %d", resp.CurrentPlaybackState.ElapsedSeconds)
	}
}

func TestEffectivePlayback_EmptyPlaylistAndFutureEpoch(t *testing.T) {
	syncRepo := newFakeSyncRepo()
	mediaRepo := newFakeMediaRepo()
	windowRepo := newFakeWindowRepo()
	playlistRepo := newFakePlaylistRepo()

	syncSvc := service.NewSyncService(syncRepo, mediaRepo, nil)
	engine := service.NewPlaybackEngine()
	playbackSvc := service.NewPlaybackService(windowRepo, playlistRepo, engine, syncSvc)

	// Window with future epoch (starts at 12:00:00)
	futureEpoch := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	window := &domain.Window{ID: 3, CycleDurationSeconds: 18000, EpochStartTime: futureEpoch}
	_ = windowRepo.Create(context.Background(), window)

	mediaSync := &domain.MediaItem{ID: 99, Type: domain.MediaTypeImage}
	_ = mediaRepo.Create(context.Background(), mediaSync)

	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)

	// When sync is active, it overrides even future epoch or empty playlist!
	_ = syncRepo.Create(context.Background(), &domain.SyncEvent{
		MediaItemID:     99,
		DurationSeconds: 60,
		StartedAt:       now,
		EndsAt:          now.Add(60 * time.Second),
		Status:          domain.SyncStatusActive,
		Media:           mediaSync,
	})

	respSync, err := playbackSvc.CalculateAt(context.Background(), 3, now.Add(10*time.Second))
	if err != nil {
		t.Fatalf("calculation failed: %v", err)
	}
	if respSync.CurrentPlaybackState.Mode != domain.PlaybackModeSync {
		t.Fatalf("expected sync override, got %s", respSync.CurrentPlaybackState.Mode)
	}

	// When sync is expired (at 10:02:00), it returns inactive (future epoch + empty playlist)
	respExpired, err := playbackSvc.CalculateAt(context.Background(), 3, now.Add(120*time.Second))
	if err != nil {
		t.Fatalf("calculation failed: %v", err)
	}
	if respExpired.CurrentPlaybackState.Mode != domain.PlaybackModeInactive {
		t.Fatalf("expected inactive mode, got %s", respExpired.CurrentPlaybackState.Mode)
	}
	if respExpired.CurrentPlaybackState.Started {
		t.Fatal("expected started=false for future epoch")
	}
}
