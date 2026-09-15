package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/service"
	"github.com/gin-gonic/gin"
)

// PlaylistEventNotifier is an interface that allows the playlist handler to
// notify interested parties (e.g., the WebSocket hub) of playlist mutations
// without depending on the ws package directly.
type PlaylistEventNotifier interface {
	NotifyPlaylistUpdated(windowID int, playlistData any)
}

// PlaylistHandler handles HTTP requests for window playlist management.
type PlaylistHandler struct {
	svc      service.PlaylistService
	notifier PlaylistEventNotifier
}

// NewPlaylistHandler creates a new PlaylistHandler. The notifier is optional
// (may be nil) — if nil, no real-time notifications are sent.
func NewPlaylistHandler(svc service.PlaylistService, notifier PlaylistEventNotifier) *PlaylistHandler {
	return &PlaylistHandler{svc: svc, notifier: notifier}
}

// notify is a helper that broadcasts a playlist update if a notifier is wired.
func (h *PlaylistHandler) notify(windowID int, data any) {
	if h.notifier != nil {
		go h.notifier.NotifyPlaylistUpdated(windowID, data)
	}
}

// GetByWindow godoc
// GET /api/windows/:id/playlist
// Returns the ordered playlist for the specified window.
func (h *PlaylistHandler) GetByWindow(c *gin.Context) {
	windowID, ok := parseID(c, "id")
	if !ok {
		return
	}

	playlist, err := h.svc.GetByWindowID(c.Request.Context(), windowID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": playlist})
}

// AddItem godoc
// POST /api/windows/:id/playlist
// Appends a media item to the window's playlist.
//
// Request body:
//
//	{
//	  "media_id": 3,
//	  "duration_seconds": 15   // optional; defaults to media's default_duration_seconds
//	}
func (h *PlaylistHandler) AddItem(c *gin.Context) {
	windowID, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req service.AddPlaylistItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("VALIDATION_ERROR", "Invalid JSON body: "+err.Error()))
		return
	}

	item, err := h.svc.AddItem(c.Request.Context(), windowID, &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": item})

	// Notify WS clients of updated playlist
	h.notifyRefreshedPlaylist(windowID)
}

// DeleteItem godoc
// DELETE /api/windows/:id/playlist/:itemId
// Removes a playlist item from the window's playlist and compacts positions.
func (h *PlaylistHandler) DeleteItem(c *gin.Context) {
	windowID, ok := parseID(c, "id")
	if !ok {
		return
	}
	itemID, ok := parseID(c, "itemId")
	if !ok {
		return
	}

	if err := h.svc.DeleteItem(c.Request.Context(), windowID, itemID); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Playlist item removed successfully"})

	// Notify WS clients of updated playlist
	h.notifyRefreshedPlaylist(windowID)
}

// Reorder godoc
// PATCH /api/windows/:id/playlist/reorder
// Reorders a window's playlist items.
//
// Request body:
//
//	{
//	  "item_ids": [5, 2, 8, 1]   // complete ordered list of all playlist item IDs
//	}
func (h *PlaylistHandler) Reorder(c *gin.Context) {
	windowID, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req service.ReorderPlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody("VALIDATION_ERROR", "Invalid JSON body: "+err.Error()))
		return
	}

	playlist, err := h.svc.Reorder(c.Request.Context(), windowID, &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": playlist})

	// Notify WS clients with the reordered playlist
	h.notify(windowID, playlist)
}

// notifyRefreshedPlaylist fetches the full playlist and sends it via the notifier.
func (h *PlaylistHandler) notifyRefreshedPlaylist(windowID int) {
	if h.notifier == nil {
		return
	}
	// Fetch fresh playlist in the background with a detached context so it doesn't
	// fail when the HTTP request context terminates.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		playlist, err := h.svc.GetByWindowID(ctx, windowID)
		if err != nil {
			return
		}
		h.notifier.NotifyPlaylistUpdated(windowID, playlist)
	}()
}
