package domain

import "time"

// MediaType defines the supported media formats
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
	MediaTypeBlank MediaType = "blank"
)

// Window represents an independent display window/screen
type Window struct {
	ID                   int            `json:"id"`
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	CycleDurationSeconds int            `json:"cycle_duration_seconds"` // 5 hours = 18,000s
	EpochStartTime       time.Time      `json:"epoch_start_time"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	PlaylistItems        []PlaylistItem `json:"playlist_items,omitempty"`
}

// MediaItem represents a media asset in the global media library
type MediaItem struct {
	ID                     int       `json:"id"`
	Title                  string    `json:"title"`
	Type                   MediaType `json:"type"` // "image", "video", "blank"
	URL                    string    `json:"url"`
	DefaultDurationSeconds int       `json:"default_duration_seconds"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// PlaylistItem represents an entry in a specific window's sequence
type PlaylistItem struct {
	ID              int        `json:"id"`
	WindowID        int        `json:"window_id"`
	MediaItemID     int        `json:"media_item_id"`
	Position        int        `json:"position"` // Deterministic ordering (1, 2, 3...)
	DurationSeconds int        `json:"duration_seconds"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Media           *MediaItem `json:"media,omitempty"`
}

// SyncStatus defines the status of a sync playback event
type SyncStatus string

const (
	SyncStatusActive    SyncStatus = "active"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusCancelled SyncStatus = "cancelled"
)

// SyncEvent represents a global synchronized playback override event
type SyncEvent struct {
	ID              int        `json:"id"`
	MediaItemID     int        `json:"media_item_id"`
	DurationSeconds int        `json:"duration_seconds"`
	StartedAt       time.Time  `json:"started_at"`
	EndsAt          time.Time  `json:"ends_at"`
	TriggeredBy     string     `json:"triggered_by"`
	Status          SyncStatus `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	Media           *MediaItem `json:"media,omitempty"`
}

// PlaybackMode defines the playback operating mode
const (
	PlaybackModeNormal   = "normal"
	PlaybackModeSync     = "sync"
	PlaybackModeInactive = "inactive"
)

// ActiveSyncState represents the currently active sync state in memory
type ActiveSyncState struct {
	Active           bool       `json:"active"`
	IsActive         bool       `json:"is_active,omitempty"`
	SyncID           int        `json:"sync_id,omitempty"`
	Media            *MediaItem `json:"media,omitempty"`
	DurationSeconds  int        `json:"duration_seconds,omitempty"`
	StartedAt        time.Time  `json:"started_at,omitempty"`
	EndsAt           time.Time  `json:"ends_at,omitempty"`
	RemainingSeconds int        `json:"remaining_seconds,omitempty"`
	ServerTime       time.Time  `json:"server_time,omitempty"`
}

// PlaybackState is the result of a single deterministic calculation by the
// PlaybackEngine for a given window, playlist, and wall-clock instant.
//
// All duration/offset fields use integer seconds.  Nanosecond arithmetic is
// performed internally by the engine; fractional seconds are not exposed in
// the API because the standard cycle is 5 hours and sub-second precision adds
// no practical value.
type PlaybackState struct {
	// Mode indicates whether playback is "normal", "sync", or "inactive".
	Mode string `json:"mode"`

	// SyncEventID is populated when Mode == "sync".
	SyncEventID int `json:"sync_event_id,omitempty"`

	// Started is false when now < epoch_start_time (cycle has not begun yet).
	Started bool `json:"started"`

	// Active is false when the playlist is empty (no item to display).
	Active bool `json:"active"`

	// PlaylistEmpty is true when the playlist has zero items.
	PlaylistEmpty bool `json:"playlist_empty"`

	// WindowID mirrors the window this state belongs to.
	WindowID int `json:"window_id"`

	// --- populated only when Active == true ---

	// PlaylistItemID is the database ID of the currently active playlist row.
	PlaylistItemID int `json:"playlist_item_id,omitempty"`

	// Position is the 1-based position of the item in the playlist sequence.
	Position int `json:"position,omitempty"`

	// Media is the full media asset currently being displayed.
	Media *MediaItem `json:"media,omitempty"`

	// ElapsedSeconds is how many seconds into the current item we are.
	ElapsedSeconds int `json:"elapsed_seconds,omitempty"`

	// RemainingSeconds is how many seconds remain for the current item.
	RemainingSeconds int `json:"remaining_seconds,omitempty"`

	// CyclePositionSeconds is our offset within the current 5-hour cycle.
	CyclePositionSeconds int `json:"cycle_position_seconds,omitempty"`

	// TotalPlaylistDuration is the sum of all active playlist item durations (seconds).
	TotalPlaylistDuration int `json:"total_playlist_duration,omitempty"`

	// CalculatedAt is the wall-clock instant used for the calculation (UTC).
	CalculatedAt time.Time `json:"calculated_at"`
}

// Standard API Response Envelopes
type ResponseEnvelope struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Error   *AppError `json:"error,omitempty"`
}
