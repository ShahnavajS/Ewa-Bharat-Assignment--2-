package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────────
// Hub configuration
// ─────────────────────────────────────────────────────────────────────────────

const (
	// timeSyncInterval controls how often the hub broadcasts TIME_SYNC to all
	// subscribed clients. 5 seconds balances accuracy vs. network overhead.
	timeSyncInterval = 5 * time.Second
)

// ─────────────────────────────────────────────────────────────────────────────
// Hub — manages active WebSocket clients, subscriptions, and event broadcasts.
// ─────────────────────────────────────────────────────────────────────────────

// Hub coordinates all WebSocket client connections, manages subscriptions by
// display window ID, and handles thread-safe broadcasts.
type Hub struct {
	// clients is the set of all connected clients.
	clients map[*Client]bool

	// windowClients maps windowID → set of subscribed clients.
	windowClients map[int]map[*Client]bool

	// broadcast channel for messages targeted at a specific window.
	broadcast chan *windowMessage

	// globalBroadcast channel for messages sent to ALL connected clients.
	globalBroadcast chan []byte

	// stop channel signals the Run loop to terminate.
	stop chan struct{}

	// done is closed when Run() has fully exited.
	done chan struct{}

	// playbackSvc is used to compute STATE_SNAPSHOT on subscription.
	playbackSvc    service.PlaybackService
	allowedOrigins map[string]struct{}

	// mu protects clients, windowClients, and stopped.
	mu      sync.RWMutex
	stopped bool
}

// windowMessage carries a pre-serialized message destined for all clients
// subscribed to a specific window.
type windowMessage struct {
	windowID int
	data     []byte
}

// NewHub creates a new Hub wired to the given PlaybackService.
func NewHub(playbackSvc service.PlaybackService, allowedOrigins ...string) *Hub {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		originSet[origin] = struct{}{}
	}
	return &Hub{
		clients:         make(map[*Client]bool),
		windowClients:   make(map[int]map[*Client]bool),
		broadcast:       make(chan *windowMessage, 128),
		globalBroadcast: make(chan []byte, 128),
		stop:            make(chan struct{}),
		done:            make(chan struct{}),
		playbackSvc:     playbackSvc,
		allowedOrigins:  originSet,
	}
}

func (h *Hub) originAllowed(origin string) bool {
	if origin == "" || len(h.allowedOrigins) == 0 {
		return true // Native clients do not send an Origin header.
	}
	_, ok := h.allowedOrigins[origin]
	return ok
}

// Run starts the hub's main event loop. It must be called in a goroutine.
// It exits cleanly when Shutdown() is called.
func (h *Hub) Run() {
	defer close(h.done)

	timeSyncTicker := time.NewTicker(timeSyncInterval)
	defer timeSyncTicker.Stop()

	for {
		select {
		case msg := <-h.broadcast:
			h.broadcastToWindow(msg.windowID, msg.data)

		case data := <-h.globalBroadcast:
			h.broadcastGlobal(data)

		case <-timeSyncTicker.C:
			h.sendTimeSync()

		case <-h.stop:
			return
		}
	}
}

// Shutdown signals the hub to stop, disconnects all connected clients,
// and waits for the Run loop to exit.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return
	}
	h.stopped = true
	close(h.stop)

	// Close and unregister all connected clients cleanly
	for client := range h.clients {
		h.removeClient(client)
	}
	h.mu.Unlock()

	<-h.done
}

// IsStopped returns true if the hub has been shut down.
func (h *Hub) IsStopped() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.stopped
}

// RegisterClient adds a client to the hub in a thread-safe manner.
func (h *Hub) RegisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		client.Close()
		return
	}
	h.clients[client] = true
	slog.Info("ws: client registered", "total_clients", len(h.clients))
}

// UnregisterClient removes a client from the hub in a thread-safe manner.
func (h *Hub) UnregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		h.removeClient(client)
		slog.Info("ws: client unregistered", "total_clients", len(h.clients))
	}
}

// ClientCount returns the number of currently connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// WindowClientCount returns the number of clients subscribed to a specific window.
func (h *Hub) WindowClientCount(windowID int) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.windowClients[windowID])
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers (mu must be held for state mutations)
// ─────────────────────────────────────────────────────────────────────────────

// removeClient removes a client from all data structures and closes it.
// Caller MUST hold h.mu.Lock().
func (h *Hub) removeClient(client *Client) {
	wid := client.WindowID()
	if wid != 0 {
		if subs, ok := h.windowClients[wid]; ok {
			delete(subs, client)
			if len(subs) == 0 {
				delete(h.windowClients, wid)
			}
		}
	}
	delete(h.clients, client)
	client.Close()
}

// subscribeClient associates a client with a window, removing any previous
// subscription. It then sends an initial STATE_SNAPSHOT.
func (h *Hub) subscribeClient(client *Client, windowID int, playbackState any) {
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return
	}

	// Remove from previous window subscription if any
	prevWid := client.WindowID()
	if prevWid != 0 && prevWid != windowID {
		if subs, ok := h.windowClients[prevWid]; ok {
			delete(subs, client)
			if len(subs) == 0 {
				delete(h.windowClients, prevWid)
			}
		}
	}

	// Associate with new window
	client.SetWindowID(windowID)
	if h.windowClients[windowID] == nil {
		h.windowClients[windowID] = make(map[*Client]bool)
	}
	h.windowClients[windowID][client] = true
	h.mu.Unlock()

	slog.Info("ws: client subscribed", "window_id", windowID)

	// Send initial STATE_SNAPSHOT
	msg := NewStateSnapshot(windowID, playbackState)
	client.SendMessage(msg)
}

// handleInbound processes a parsed client message.
func (h *Hub) handleInbound(client *Client, msg *InboundMessage) {
	switch msg.Type {
	case MsgTypeSubscribe:
		if msg.WindowID <= 0 {
			client.SendMessage(NewErrorMessage("VALIDATION_ERROR", "window_id must be a positive integer"))
			return
		}

		// Validate window exists via playback service
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := h.playbackSvc.GetWindowWithPlayback(ctx, msg.WindowID)
		if err != nil {
			client.SendMessage(NewErrorMessage("WINDOW_NOT_FOUND", "Window not found"))
			return
		}

		h.subscribeClient(client, msg.WindowID, resp)

	default:
		client.SendMessage(NewErrorMessage("UNKNOWN_MESSAGE_TYPE",
			"Unknown message type: "+msg.Type))
	}
}

// broadcastToWindow sends pre-serialized data to all clients subscribed to the
// given window. Slow clients that cannot accept the message are disconnected.
func (h *Hub) broadcastToWindow(windowID int, data []byte) {
	h.mu.RLock()
	subs, ok := h.windowClients[windowID]
	if !ok || len(subs) == 0 {
		h.mu.RUnlock()
		return
	}

	// Copy subscriber slice so we can release the lock before writing
	targets := make([]*Client, 0, len(subs))
	for client := range subs {
		targets = append(targets, client)
	}
	h.mu.RUnlock()

	for _, client := range targets {
		if !client.SendRaw(data) {
			h.UnregisterClient(client)
		}
	}
}

// broadcastGlobal sends pre-serialized data to all connected clients.
func (h *Hub) broadcastGlobal(data []byte) {
	h.mu.RLock()
	targets := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		targets = append(targets, client)
	}
	h.mu.RUnlock()

	for _, client := range targets {
		if !client.SendRaw(data) {
			h.UnregisterClient(client)
		}
	}
}

// sendTimeSync broadcasts a TIME_SYNC message to all subscribed clients.
func (h *Hub) sendTimeSync() {
	msg := NewTimeSync()
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("ws: failed to marshal TIME_SYNC", "error", err)
		return
	}

	h.mu.RLock()
	targets := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		if client.WindowID() != 0 {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range targets {
		if !client.SendRaw(data) {
			h.UnregisterClient(client)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Public domain event publishers — called from service/handler layer
// ─────────────────────────────────────────────────────────────────────────────

// NotifyPlaylistUpdated is called by the playlist handler/service after a
// playlist mutation (add, delete, reorder). It broadcasts the updated playlist
// to all clients subscribed to the affected window.
func (h *Hub) NotifyPlaylistUpdated(windowID int, playlistData any) {
	h.mu.RLock()
	if h.stopped {
		h.mu.RUnlock()
		return
	}
	h.mu.RUnlock()

	msg := NewPlaylistUpdated(windowID, playlistData)
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("ws: failed to marshal PLAYLIST_UPDATED", "error", err)
		return
	}

	select {
	case h.broadcast <- &windowMessage{windowID: windowID, data: data}:
	default:
		// Drop or process synchronously if channel full
		go h.broadcastToWindow(windowID, data)
	}
}

// NotifySyncStarted broadcasts a SYNC_STARTED event to ALL connected clients.
func (h *Hub) NotifySyncStarted(syncData any) {
	h.mu.RLock()
	if h.stopped {
		h.mu.RUnlock()
		return
	}
	h.mu.RUnlock()

	msg := NewSyncStarted(syncData)
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("ws: failed to marshal SYNC_STARTED", "error", err)
		return
	}

	select {
	case h.globalBroadcast <- data:
	default:
		go h.broadcastGlobal(data)
	}
}

// NotifySyncEnded broadcasts a SYNC_ENDED event to ALL connected clients.
func (h *Hub) NotifySyncEnded(syncData any) {
	h.mu.RLock()
	if h.stopped {
		h.mu.RUnlock()
		return
	}
	h.mu.RUnlock()

	msg := NewSyncEnded(syncData)
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("ws: failed to marshal SYNC_ENDED", "error", err)
		return
	}

	select {
	case h.globalBroadcast <- data:
	default:
		go h.broadcastGlobal(data)
	}
}
