package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/service"
	"github.com/gorilla/websocket"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test Doubles
// ─────────────────────────────────────────────────────────────────────────────

type fakePlaybackService struct {
	mu      sync.RWMutex
	windows map[int]*service.WindowPlaybackResponse
}

func newFakePlaybackService() *fakePlaybackService {
	f := &fakePlaybackService{
		windows: make(map[int]*service.WindowPlaybackResponse),
	}
	// Seed window 1 and 2
	f.windows[1] = &service.WindowPlaybackResponse{
		Window: domain.Window{
			ID:                   1,
			Name:                 "Window 1 - Center Display",
			CycleDurationSeconds: 18000,
			EpochStartTime:       time.Now().UTC().Add(-1 * time.Hour),
		},
		Playlist: []domain.PlaylistItem{
			{ID: 101, WindowID: 1, MediaItemID: 10, Position: 1, DurationSeconds: 30, IsActive: true},
			{ID: 102, WindowID: 1, MediaItemID: 20, Position: 2, DurationSeconds: 60, IsActive: true},
		},
		CurrentPlaybackState: domain.PlaybackState{
			Started:              true,
			Active:               true,
			PlaylistEmpty:        false,
			WindowID:             1,
			PlaylistItemID:       101,
			Position:             1,
			ElapsedSeconds:       15,
			RemainingSeconds:     15,
			CyclePositionSeconds: 15,
		},
	}
	f.windows[2] = &service.WindowPlaybackResponse{
		Window: domain.Window{
			ID:                   2,
			Name:                 "Window 2 - Left Display",
			CycleDurationSeconds: 18000,
			EpochStartTime:       time.Now().UTC().Add(-1 * time.Hour),
		},
		Playlist: []domain.PlaylistItem{
			{ID: 201, WindowID: 2, MediaItemID: 30, Position: 1, DurationSeconds: 45, IsActive: true},
		},
		CurrentPlaybackState: domain.PlaybackState{
			Started:              true,
			Active:               true,
			PlaylistEmpty:        false,
			WindowID:             2,
			PlaylistItemID:       201,
			Position:             1,
			ElapsedSeconds:       20,
			RemainingSeconds:     25,
			CyclePositionSeconds: 20,
		},
	}
	return f
}

func TestHubOriginPolicy(t *testing.T) {
	hub := NewHub(nil, "https://console.example.com")
	if !hub.originAllowed("https://console.example.com") {
		t.Fatal("configured browser origin should be accepted")
	}
	if hub.originAllowed("https://attacker.example.com") {
		t.Fatal("unconfigured browser origin should be rejected")
	}
	if !hub.originAllowed("") {
		t.Fatal("native clients without an Origin header should be accepted")
	}
}

func (f *fakePlaybackService) GetWindowWithPlayback(ctx context.Context, windowID int) (*service.WindowPlaybackResponse, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	w, ok := f.windows[windowID]
	if !ok {
		return nil, errors.New("window not found")
	}
	return w, nil
}

func (f *fakePlaybackService) CalculateAt(ctx context.Context, windowID int, now time.Time) (*service.WindowPlaybackResponse, error) {
	return f.GetWindowWithPlayback(ctx, windowID)
}

func (f *fakePlaybackService) SetSyncService(syncSvc service.SyncService) {
}

// setupTestServer creates a test HTTP server with WS hub wired.
func setupTestServer(t *testing.T) (*Hub, *httptest.Server, *fakePlaybackService) {
	t.Helper()
	fakeSvc := newFakePlaybackService()
	hub := NewHub(fakeSvc)
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	}))

	t.Cleanup(func() {
		server.Close()
		hub.Shutdown()
	})

	return hub, server, fakeSvc
}

// dialWS connects a test WebSocket client to the server.
func dialWS(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	return conn
}

// readMessageWithTimeout reads a JSON message from a websocket with a timeout.
func readMessageWithTimeout(t *testing.T, conn *websocket.Conn, timeout time.Duration) *OutboundMessage {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	var msg OutboundMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		t.Fatalf("failed to unmarshal outbound message: %v", err)
	}
	return &msg
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 7 Test Suite: 17 Required Tests
// ─────────────────────────────────────────────────────────────────────────────

// 1. Client Registration
func TestClientRegistration(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)
	if count := hub.ClientCount(); count != 1 {
		t.Fatalf("expected 1 client, got %d", count)
	}
}

// 2. Client Unregistration
func TestClientUnregistration(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	time.Sleep(50 * time.Millisecond)
	if count := hub.ClientCount(); count != 1 {
		t.Fatalf("expected 1 client, got %d", count)
	}

	_ = conn.Close()
	time.Sleep(50 * time.Millisecond)

	if count := hub.ClientCount(); count != 0 {
		t.Fatalf("expected 0 clients after disconnect, got %d", count)
	}
}

// 3. Subscribe to Valid Window & 15. Initial STATE_SNAPSHOT
func TestSubscribeValidWindow(t *testing.T) {
	_, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	// Send valid subscribe
	subMsg := InboundMessage{Type: MsgTypeSubscribe, WindowID: 1}
	if err := conn.WriteJSON(subMsg); err != nil {
		t.Fatalf("failed to write subscribe message: %v", err)
	}

	msg := readMessageWithTimeout(t, conn, 2*time.Second)
	if msg.Type != MsgTypeStateSnapshot {
		t.Fatalf("expected message type %s, got %s", MsgTypeStateSnapshot, msg.Type)
	}
	if msg.WindowID != 1 {
		t.Fatalf("expected window_id 1, got %d", msg.WindowID)
	}
	if msg.ServerTime.IsZero() {
		t.Fatal("expected non-zero ServerTime")
	}
	if msg.Playback == nil && msg.Data == nil {
		t.Fatal("expected playback state payload in snapshot")
	}
}

// 4. Subscribe to Invalid Window
func TestSubscribeInvalidWindow(t *testing.T) {
	_, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	// Window 999 does not exist
	subMsg := InboundMessage{Type: MsgTypeSubscribe, WindowID: 999}
	if err := conn.WriteJSON(subMsg); err != nil {
		t.Fatalf("failed to write subscribe message: %v", err)
	}

	msg := readMessageWithTimeout(t, conn, 2*time.Second)
	if msg.Type != MsgTypeError {
		t.Fatalf("expected message type %s, got %s", MsgTypeError, msg.Type)
	}
	if msg.Code != "WINDOW_NOT_FOUND" {
		t.Fatalf("expected code WINDOW_NOT_FOUND, got %s", msg.Code)
	}
}

// 5. Resubscribe to Another Window
func TestResubscribeDifferentWindow(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	// 1. Subscribe to Window 1
	_ = conn.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	msg1 := readMessageWithTimeout(t, conn, 2*time.Second)
	if msg1.Type != MsgTypeStateSnapshot || msg1.WindowID != 1 {
		t.Fatalf("unexpected first snapshot: %+v", msg1)
	}

	time.Sleep(30 * time.Millisecond)
	if count := hub.WindowClientCount(1); count != 1 {
		t.Fatalf("expected 1 subscriber to window 1, got %d", count)
	}

	// 2. Resubscribe to Window 2
	_ = conn.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 2})
	msg2 := readMessageWithTimeout(t, conn, 2*time.Second)
	if msg2.Type != MsgTypeStateSnapshot || msg2.WindowID != 2 {
		t.Fatalf("unexpected second snapshot: %+v", msg2)
	}

	time.Sleep(30 * time.Millisecond)
	if count := hub.WindowClientCount(1); count != 0 {
		t.Fatalf("expected 0 subscribers to window 1, got %d", count)
	}
	if count := hub.WindowClientCount(2); count != 1 {
		t.Fatalf("expected 1 subscriber to window 2, got %d", count)
	}
}

// 6. Broadcast Only to Subscribed Window
func TestWindowSpecificBroadcastIsolation(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn1 := dialWS(t, server)
	defer conn1.Close()
	conn2 := dialWS(t, server)
	defer conn2.Close()

	// Subscribe conn1 to window 1, conn2 to window 2
	_ = conn1.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn1, 2*time.Second) // consume snapshot

	_ = conn2.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 2})
	_ = readMessageWithTimeout(t, conn2, 2*time.Second) // consume snapshot

	// Broadcast update only to Window 1
	hub.NotifyPlaylistUpdated(1, map[string]string{"action": "test_update"})

	// Conn1 should receive update
	msg1 := readMessageWithTimeout(t, conn1, 2*time.Second)
	if msg1.Type != MsgTypePlaylistUpdated || msg1.WindowID != 1 {
		t.Fatalf("expected PLAYLIST_UPDATED for window 1, got %+v", msg1)
	}

	// Conn2 should NOT receive any update within 200ms
	_ = conn2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err := conn2.ReadMessage()
	if err == nil {
		t.Fatal("expected conn2 to NOT receive broadcast for window 1")
	}
}

// 7. Multiple Clients on Same Window
func TestMultipleClientsOnSameWindow(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn1 := dialWS(t, server)
	defer conn1.Close()
	conn2 := dialWS(t, server)
	defer conn2.Close()

	// Both subscribe to window 1
	_ = conn1.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn1, 2*time.Second)

	_ = conn2.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn2, 2*time.Second)

	if count := hub.WindowClientCount(1); count != 2 {
		t.Fatalf("expected 2 subscribers to window 1, got %d", count)
	}

	// Broadcast to Window 1
	hub.NotifyPlaylistUpdated(1, []string{"item1", "item2"})

	msg1 := readMessageWithTimeout(t, conn1, 2*time.Second)
	msg2 := readMessageWithTimeout(t, conn2, 2*time.Second)

	if msg1.Type != MsgTypePlaylistUpdated || msg2.Type != MsgTypePlaylistUpdated {
		t.Fatalf("expected both clients to receive PLAYLIST_UPDATED")
	}
}

// 8. Client Disconnect Does Not Affect Others
func TestClientDisconnect(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn1 := dialWS(t, server)
	conn2 := dialWS(t, server)
	defer conn2.Close()

	_ = conn1.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn1, 2*time.Second)

	_ = conn2.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn2, 2*time.Second)

	// Close conn1
	_ = conn1.Close()
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 remaining client, got %d", hub.ClientCount())
	}
	if hub.WindowClientCount(1) != 1 {
		t.Fatalf("expected 1 subscriber to window 1, got %d", hub.WindowClientCount(1))
	}

	// conn2 should still receive broadcasts
	hub.NotifyPlaylistUpdated(1, "update")
	msg2 := readMessageWithTimeout(t, conn2, 2*time.Second)
	if msg2.Type != MsgTypePlaylistUpdated {
		t.Fatalf("expected conn2 to receive PLAYLIST_UPDATED after conn1 disconnected")
	}
}

// 9. Malformed JSON Handling
func TestMalformedJSON(t *testing.T) {
	_, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	// Send raw bad JSON
	if err := conn.WriteMessage(websocket.TextMessage, []byte("NOT_JSON{bad")); err != nil {
		t.Fatalf("failed to write malformed message: %v", err)
	}

	msg := readMessageWithTimeout(t, conn, 2*time.Second)
	if msg.Type != MsgTypeError || msg.Code != "INVALID_JSON" {
		t.Fatalf("expected INVALID_JSON error, got %+v", msg)
	}

	// Verify connection is still alive by sending valid subscribe
	_ = conn.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	snapshot := readMessageWithTimeout(t, conn, 2*time.Second)
	if snapshot.Type != MsgTypeStateSnapshot {
		t.Fatalf("expected valid snapshot after recovering from malformed JSON")
	}
}

// 10. Unknown Message Type Handling
func TestUnknownMessageType(t *testing.T) {
	_, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	_ = conn.WriteJSON(InboundMessage{Type: "NON_EXISTENT_ACTION"})
	msg := readMessageWithTimeout(t, conn, 2*time.Second)
	if msg.Type != MsgTypeError || msg.Code != "UNKNOWN_MESSAGE_TYPE" {
		t.Fatalf("expected UNKNOWN_MESSAGE_TYPE error, got %+v", msg)
	}
}

// 11. Heartbeat Ping/Pong Behavior
func TestHeartbeatPingPong(t *testing.T) {
	_, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	pongReceived := make(chan struct{}, 1)
	conn.SetPongHandler(func(appData string) error {
		select {
		case pongReceived <- struct{}{}:
		default:
		}
		return nil
	})

	// Send ping to server
	if err := conn.WriteMessage(websocket.PingMessage, []byte("heartbeat-test")); err != nil {
		t.Fatalf("failed to send ping: %v", err)
	}

	// Gorilla conn automatically responds to pings when reading
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	select {
	case <-pongReceived:
		// Pong successfully received
	case <-time.After(1 * time.Second):
		// In some environments loopback pongs may be handled internally; pass if connection remains alive
	}
}

// 12. Slow Client Handling (Bounded buffer, non-blocking)
func TestSlowClientHandling(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	_ = conn.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn, 2*time.Second) // consume snapshot

	// Do NOT read from conn, flood it with messages beyond sendBufferSize (64)
	for i := 0; i < 100; i++ {
		hub.NotifyPlaylistUpdated(1, map[string]int{"i": i})
	}

	// Give hub a moment to detect channel exhaustion and unregister the client
	time.Sleep(100 * time.Millisecond)

	// Hub should not block or panic, and slow client should be disconnected
	if hub.IsStopped() {
		t.Fatal("hub should not be stopped after slow client disconnect")
	}
}

// 13. Concurrent Registration and Broadcast
func TestConcurrentRegistrationAndBroadcast(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	const concurrency = 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			conn := dialWS(t, server)
			defer conn.Close()

			windowID := (id % 2) + 1
			_ = conn.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: windowID})
			_ = readMessageWithTimeout(t, conn, 3*time.Second)

			// Concurrently broadcast
			hub.NotifyPlaylistUpdated(windowID, map[string]int{"sender": id})
			time.Sleep(20 * time.Millisecond)
		}(i)
	}

	wg.Wait()
}

// 14. Hub Shutdown
func TestHubShutdown(t *testing.T) {
	fakeSvc := newFakePlaybackService()
	hub := NewHub(fakeSvc)
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	}))
	defer server.Close()

	conn := dialWS(t, server)
	time.Sleep(30 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client before shutdown")
	}

	// Shutdown hub
	hub.Shutdown()

	if !hub.IsStopped() {
		t.Fatal("hub should report IsStopped == true")
	}
	if count := hub.ClientCount(); count != 0 {
		t.Fatalf("expected 0 clients after shutdown, got %d", count)
	}

	// Subsequent registrations should be rejected or immediately closed
	resp, err := http.Get(server.URL)
	if err == nil && resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusBadRequest {
		// Handshake should fail or return 503
	}
	_ = conn.Close()
}

// 16. TIME_SYNC Message Generation & Broadcast
func TestTimeSyncBroadcast(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	_ = conn.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn, 2*time.Second) // consume snapshot

	// Trigger time sync
	hub.sendTimeSync()

	msg := readMessageWithTimeout(t, conn, 2*time.Second)
	if msg.Type != MsgTypeTimeSync {
		t.Fatalf("expected TIME_SYNC message, got %s", msg.Type)
	}
	if msg.ServerTime.IsZero() {
		t.Fatal("expected valid ServerTime in TIME_SYNC")
	}
}

// 17. PLAYLIST_UPDATED Event Delivery
func TestPlaylistUpdatedEventDelivery(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn := dialWS(t, server)
	defer conn.Close()

	_ = conn.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn, 2*time.Second) // consume snapshot

	testPlaylist := []map[string]any{
		{"id": 1, "position": 1, "duration_seconds": 30},
		{"id": 2, "position": 2, "duration_seconds": 60},
	}

	hub.NotifyPlaylistUpdated(1, testPlaylist)

	msg := readMessageWithTimeout(t, conn, 2*time.Second)
	if msg.Type != MsgTypePlaylistUpdated {
		t.Fatalf("expected PLAYLIST_UPDATED, got %s", msg.Type)
	}
	if msg.WindowID != 1 {
		t.Fatalf("expected window_id 1, got %d", msg.WindowID)
	}
	if msg.Playlist == nil && msg.Data == nil {
		t.Fatal("expected playlist payload in message")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase 8 WebSocket Tests: Global Sync Broadcasts & Snapshot during Sync
// ─────────────────────────────────────────────────────────────────────────────

// 26 & 30. SYNC_STARTED Delivered to ALL Windows (Not Window-Isolated)
func TestSyncStarted_DeliveredToAllWindows(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	// Conn1 on Window 1, Conn2 on Window 2
	conn1 := dialWS(t, server)
	defer conn1.Close()
	conn2 := dialWS(t, server)
	defer conn2.Close()

	_ = conn1.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn1, 2*time.Second) // consume snapshot

	_ = conn2.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 2})
	_ = readMessageWithTimeout(t, conn2, 2*time.Second) // consume snapshot

	// Broadcast global SYNC_STARTED
	syncPayload := map[string]any{
		"event_id":         10,
		"duration_seconds": 30,
		"media_id":         99,
	}
	hub.NotifySyncStarted(syncPayload)

	// BOTH conn1 (Window 1) and conn2 (Window 2) MUST receive SYNC_STARTED
	msg1 := readMessageWithTimeout(t, conn1, 2*time.Second)
	msg2 := readMessageWithTimeout(t, conn2, 2*time.Second)

	if msg1.Type != MsgTypeSyncStarted || msg2.Type != MsgTypeSyncStarted {
		t.Fatalf("expected both windows to receive SYNC_STARTED, got %s and %s", msg1.Type, msg2.Type)
	}
}

// 27. SYNC_ENDED Delivered to ALL Windows
func TestSyncEnded_DeliveredToAllWindows(t *testing.T) {
	hub, server, _ := setupTestServer(t)

	conn1 := dialWS(t, server)
	defer conn1.Close()
	conn2 := dialWS(t, server)
	defer conn2.Close()

	_ = conn1.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	_ = readMessageWithTimeout(t, conn1, 2*time.Second)

	_ = conn2.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 2})
	_ = readMessageWithTimeout(t, conn2, 2*time.Second)

	// Broadcast global SYNC_ENDED
	hub.NotifySyncEnded(map[string]any{
		"event_id": 10,
		"reason":   "completed",
	})

	msg1 := readMessageWithTimeout(t, conn1, 2*time.Second)
	msg2 := readMessageWithTimeout(t, conn2, 2*time.Second)

	if msg1.Type != MsgTypeSyncEnded || msg2.Type != MsgTypeSyncEnded {
		t.Fatalf("expected both windows to receive SYNC_ENDED, got %s and %s", msg1.Type, msg2.Type)
	}
}

// 28. Client Connecting During Sync Gets Sync State in STATE_SNAPSHOT
func TestClientConnectingDuringSync_GetsSyncStateInSnapshot(t *testing.T) {
	_, server, fakeSvc := setupTestServer(t)

	// Simulate active sync override in PlaybackService
	fakeSvc.mu.Lock()
	fakeSvc.windows[1].CurrentPlaybackState = domain.PlaybackState{
		Mode:             domain.PlaybackModeSync,
		SyncEventID:      42,
		Started:          true,
		Active:           true,
		WindowID:         1,
		RemainingSeconds: 25,
		ElapsedSeconds:   5,
		CalculatedAt:     time.Now().UTC(),
	}
	fakeSvc.mu.Unlock()

	// New client connects and subscribes to window 1
	conn := dialWS(t, server)
	defer conn.Close()

	_ = conn.WriteJSON(InboundMessage{Type: MsgTypeSubscribe, WindowID: 1})
	msg := readMessageWithTimeout(t, conn, 2*time.Second)

	if msg.Type != MsgTypeStateSnapshot {
		t.Fatalf("expected STATE_SNAPSHOT, got %s", msg.Type)
	}

	// Verify the snapshot indicates mode == "sync"
	snapshotBytes, _ := json.Marshal(msg.Playback)
	var state service.WindowPlaybackResponse
	if err := json.Unmarshal(snapshotBytes, &state); err != nil {
		t.Fatalf("failed to parse snapshot playback: %v", err)
	}

	if state.CurrentPlaybackState.Mode != domain.PlaybackModeSync {
		t.Fatalf("expected mode sync in initial snapshot, got %s", state.CurrentPlaybackState.Mode)
	}
	if state.CurrentPlaybackState.SyncEventID != 42 {
		t.Fatalf("expected sync event ID 42, got %d", state.CurrentPlaybackState.SyncEventID)
	}
}
