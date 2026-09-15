package ws

import (
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Message type constants — stable string identifiers used across the protocol.
// ─────────────────────────────────────────────────────────────────────────────

const (
	// Client → Server message types
	MsgTypeSubscribe = "SUBSCRIBE"

	// Server → Client message types
	MsgTypeStateSnapshot   = "STATE_SNAPSHOT"
	MsgTypeTimeSync        = "TIME_SYNC"
	MsgTypePlaylistUpdated = "PLAYLIST_UPDATED"
	MsgTypeSyncStarted     = "SYNC_STARTED"
	MsgTypeSyncEnded       = "SYNC_ENDED"
	MsgTypeError           = "ERROR"
)

// ─────────────────────────────────────────────────────────────────────────────
// Inbound messages (Client → Server)
// ─────────────────────────────────────────────────────────────────────────────

// InboundMessage is the generic envelope for client-to-server messages.
type InboundMessage struct {
	Type     string `json:"type"`
	WindowID int    `json:"window_id,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Outbound messages (Server → Client)
// ─────────────────────────────────────────────────────────────────────────────

// OutboundMessage is the generic envelope for all server-to-client messages.
// It provides a consistent schema across all event types with UTC timestamps.
type OutboundMessage struct {
	Type       string    `json:"type"`
	ServerTime time.Time `json:"server_time"`
	Timestamp  time.Time `json:"timestamp"`
	WindowID   int       `json:"window_id,omitempty"`
	Playback   any       `json:"playback,omitempty"`
	Playlist   any       `json:"playlist,omitempty"`
	Sync       any       `json:"sync,omitempty"`
	Data       any       `json:"data,omitempty"`
	Code       string    `json:"code,omitempty"`
	Message    string    `json:"message,omitempty"`
}

// NewStateSnapshot creates a STATE_SNAPSHOT message for a window.
func NewStateSnapshot(windowID int, playbackState any) *OutboundMessage {
	now := time.Now().UTC()
	return &OutboundMessage{
		Type:       MsgTypeStateSnapshot,
		ServerTime: now,
		Timestamp:  now,
		WindowID:   windowID,
		Playback:   playbackState,
		Data:       playbackState,
	}
}

// NewTimeSync creates a periodic TIME_SYNC message.
func NewTimeSync() *OutboundMessage {
	now := time.Now().UTC()
	return &OutboundMessage{
		Type:       MsgTypeTimeSync,
		ServerTime: now,
		Timestamp:  now,
		Data: map[string]any{
			"server_time": now,
		},
	}
}

// NewPlaylistUpdated creates a PLAYLIST_UPDATED message for a window.
func NewPlaylistUpdated(windowID int, playlist any) *OutboundMessage {
	now := time.Now().UTC()
	return &OutboundMessage{
		Type:       MsgTypePlaylistUpdated,
		ServerTime: now,
		Timestamp:  now,
		WindowID:   windowID,
		Playlist:   playlist,
		Data:       playlist,
	}
}

// NewSyncStarted creates a SYNC_STARTED message for global broadcast.
func NewSyncStarted(syncData any) *OutboundMessage {
	now := time.Now().UTC()
	return &OutboundMessage{
		Type:       MsgTypeSyncStarted,
		ServerTime: now,
		Timestamp:  now,
		Sync:       syncData,
		Data:       syncData,
	}
}

// NewSyncEnded creates a SYNC_ENDED message for global broadcast.
func NewSyncEnded(syncData any) *OutboundMessage {
	now := time.Now().UTC()
	return &OutboundMessage{
		Type:       MsgTypeSyncEnded,
		ServerTime: now,
		Timestamp:  now,
		Sync:       syncData,
		Data:       syncData,
	}
}

// NewErrorMessage creates a structured ERROR message.
func NewErrorMessage(code, message string) *OutboundMessage {
	now := time.Now().UTC()
	return &OutboundMessage{
		Type:       MsgTypeError,
		ServerTime: now,
		Timestamp:  now,
		Code:       code,
		Message:    message,
	}
}
