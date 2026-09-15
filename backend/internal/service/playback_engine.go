// Package service contains the core business logic for the EVA Media Sequencer.
// This file implements the pure, deterministic playback engine.
//
// # Design Principles
//
//   - No database access — caller fetches window + playlist, passes them in.
//   - No goroutines, timers, tickers, or sleeps.
//   - No global or mutable state.
//   - Fully deterministic: identical inputs always yield identical outputs.
//   - All internal arithmetic uses time.Duration (nanosecond int64) to avoid
//     floating-point drift. Integer seconds are exposed in PlaybackState.
//
// # Playback Formula
//
//  1. elapsed     = now − epoch_start_time          (time.Duration, can be negative)
//  2. If elapsed < 0 → not-started state.
//  3. cyclePos    = elapsed % cycleDuration          (nanoseconds, always 0..cycle-1)
//  4. playlistPos = cyclePos % totalPlaylistDuration  (nanoseconds, always 0..total-1)
//  5. Walk playlist items in position order; the item whose cumulative duration
//     range contains playlistPos is the active item.
//  6. Boundary semantics: item owns [start, end).  At exact end → next item.
package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/domain"
)

// ─────────────────────────────────────────────────────────────────────────────
// Error sentinel values for engine validation failures.
// ─────────────────────────────────────────────────────────────────────────────

var (
	// ErrInvalidCycleDuration is returned when cycle_duration_seconds ≤ 0.
	ErrInvalidCycleDuration = errors.New("playback engine: cycle_duration_seconds must be > 0")

	// ErrInvalidPlaylistDuration is returned when any playlist item has duration ≤ 0.
	ErrInvalidPlaylistDuration = errors.New("playback engine: all playlist item duration_seconds must be > 0")

	// ErrNilMedia is returned when a playlist item has a nil Media pointer.
	ErrNilMedia = errors.New("playback engine: playlist item has nil media reference")

	// ErrUnsortedPlaylist is returned when playlist items are not strictly
	// ordered by ascending position (1, 2, 3…).
	ErrUnsortedPlaylist = errors.New("playback engine: playlist items must be ordered by ascending position")
)

// ─────────────────────────────────────────────────────────────────────────────
// PlaybackEngine — public interface
// ─────────────────────────────────────────────────────────────────────────────

// PlaybackEngine calculates the deterministic playback state for a window.
// It is a pure calculation service: no database, no timers, no goroutines.
type PlaybackEngine interface {
	// Calculate returns the playback state for the given window/playlist at `now`.
	// `playlist` must be ordered by position ASC (exactly as returned by the
	// playlist repository).  Each item must have a non-nil Media field.
	Calculate(
		window domain.Window,
		playlist []domain.PlaylistItem,
		now time.Time,
	) (domain.PlaybackState, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// playbackEngine — concrete implementation
// ─────────────────────────────────────────────────────────────────────────────

type playbackEngine struct{}

// NewPlaybackEngine returns a new stateless PlaybackEngine instance.
func NewPlaybackEngine() PlaybackEngine {
	return &playbackEngine{}
}

// Calculate implements PlaybackEngine.
func (e *playbackEngine) Calculate(
	window domain.Window,
	playlist []domain.PlaylistItem,
	now time.Time,
) (domain.PlaybackState, error) {

	calculatedAt := now.UTC()
	base := domain.PlaybackState{
		WindowID:     window.ID,
		CalculatedAt: calculatedAt,
	}

	// ── 1. Validate cycle duration ────────────────────────────────────────────
	if window.CycleDurationSeconds <= 0 {
		return base, fmt.Errorf("%w: got %d", ErrInvalidCycleDuration, window.CycleDurationSeconds)
	}

	// ── 2. Check whether the cycle has started ────────────────────────────────
	epoch := window.EpochStartTime.UTC()
	elapsed := now.UTC().Sub(epoch) // can be negative if now < epoch

	if elapsed < 0 {
		// Cycle has not started yet — return a well-defined not-started state.
		base.Started = false
		base.Active = false
		return base, nil
	}
	base.Started = true

	// ── 3. Empty playlist guard ───────────────────────────────────────────────
	if len(playlist) == 0 {
		base.Active = false
		base.PlaylistEmpty = true
		return base, nil
	}

	// ── 4. Validate playlist items (defensive; DB prevents most violations) ───
	if err := validatePlaylist(playlist); err != nil {
		return base, err
	}

	// ── 5. Compute total playlist duration ────────────────────────────────────
	var totalPlaylistNs time.Duration
	for _, item := range playlist {
		totalPlaylistNs += time.Duration(item.DurationSeconds) * time.Second
	}

	// ── 6. Cycle position (nanoseconds, always in [0, cycleDuration)) ─────────
	cycleDuration := time.Duration(window.CycleDurationSeconds) * time.Second
	cyclePos := elapsed % cycleDuration // Go % always takes sign of dividend
	// elapsed ≥ 0, so cyclePos ∈ [0, cycleDuration).

	// ── 7. Playlist position (nanoseconds, always in [0, totalPlaylist)) ──────
	// If playlist total > cycle, the cycle boundary wins (cyclePos already
	// caps at cycleDuration-1). If playlist total < cycle, the playlist repeats.
	playlistPos := cyclePos % totalPlaylistNs

	// ── 8. Walk items to find the active one ─────────────────────────────────
	var cumulative time.Duration
	for _, item := range playlist {
		itemDur := time.Duration(item.DurationSeconds) * time.Second
		itemStart := cumulative
		itemEnd := cumulative + itemDur // owns [itemStart, itemEnd)

		if playlistPos >= itemStart && playlistPos < itemEnd {
			elapsedInItem := playlistPos - itemStart
			remainingInItem := itemEnd - playlistPos - time.Nanosecond
			// remainingInItem is "time until this item ends"; we floor to seconds.

			return domain.PlaybackState{
				Started:               true,
				Active:                true,
				PlaylistEmpty:         false,
				WindowID:              window.ID,
				PlaylistItemID:        item.ID,
				Position:              item.Position,
				Media:                 item.Media,
				ElapsedSeconds:        int(elapsedInItem / time.Second),
				RemainingSeconds:      int(remainingInItem/time.Second) + 1,
				CyclePositionSeconds:  int(cyclePos / time.Second),
				TotalPlaylistDuration: int(totalPlaylistNs / time.Second),
				CalculatedAt:          calculatedAt,
			}, nil
		}

		cumulative += itemDur
	}

	// Should be unreachable: playlistPos is always < totalPlaylistNs and the
	// loop covers the full range.  Guard anyway.
	return base, fmt.Errorf("playback engine: internal error — no item matched playlistPos=%v total=%v", playlistPos, totalPlaylistNs)
}

// ─────────────────────────────────────────────────────────────────────────────
// validatePlaylist — defensive checks on engine inputs
// ─────────────────────────────────────────────────────────────────────────────

// validatePlaylist checks for conditions that should not occur in normal
// operation (the DB prevents most of them) but could arise from test fixtures
// or future code paths.
func validatePlaylist(playlist []domain.PlaylistItem) error {
	var prevPos int
	for i, item := range playlist {
		// Nil media check.
		if item.Media == nil {
			return fmt.Errorf("%w: item at index %d (id=%d)", ErrNilMedia, i, item.ID)
		}
		// Duration must be positive.
		if item.DurationSeconds <= 0 {
			return fmt.Errorf("%w: item at index %d (id=%d) has duration %d",
				ErrInvalidPlaylistDuration, i, item.ID, item.DurationSeconds)
		}
		// Positions must be strictly ascending (1, 2, 3…).
		if i > 0 && item.Position <= prevPos {
			return fmt.Errorf("%w: item at index %d has position %d ≤ previous %d",
				ErrUnsortedPlaylist, i, item.Position, prevPos)
		}
		prevPos = item.Position
	}
	return nil
}
