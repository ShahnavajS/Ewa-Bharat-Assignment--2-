// Package service_test contains unit tests for the deterministic PlaybackEngine.
//
// All tests are pure (no database, no network, no goroutines).
// The engine is exercised through the PlaybackEngine interface; concrete
// wall-clock values are passed in explicitly so results are repeatable.
package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/internal/domain"
	"github.com/eva-bharat/media-sequencer/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test fixtures / helpers
// ─────────────────────────────────────────────────────────────────────────────

var epoch = time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

// makeWindow builds a domain.Window with the given cycle (0 → default 18000s).
func makeWindow(cycleSeconds int) domain.Window {
	if cycleSeconds == 0 {
		cycleSeconds = 18000
	}
	return domain.Window{
		ID:                   1,
		Name:                 "Test Window",
		CycleDurationSeconds: cycleSeconds,
		EpochStartTime:       epoch,
	}
}

// makeMedia creates a minimal MediaItem with the given ID.
func makeMedia(id int) *domain.MediaItem {
	return &domain.MediaItem{
		ID:    id,
		Title: "media",
		Type:  domain.MediaTypeImage,
		URL:   "https://example.com/img.jpg",
	}
}

// makePlaylist builds a playlist with the given (mediaID, durationSeconds) pairs,
// assigning sequential positions starting at 1.
func makePlaylist(items ...[2]int) []domain.PlaylistItem {
	playlist := make([]domain.PlaylistItem, len(items))
	for i, pair := range items {
		mediaID, dur := pair[0], pair[1]
		playlist[i] = domain.PlaylistItem{
			ID:              100 + i,
			WindowID:        1,
			MediaItemID:     mediaID,
			Position:        i + 1,
			DurationSeconds: dur,
			IsActive:        true,
			Media:           makeMedia(mediaID),
		}
	}
	return playlist
}

// at returns `epoch + offset` as a UTC time.
func at(offset time.Duration) time.Time {
	return epoch.Add(offset).UTC()
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 1 — Empty playlist
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_EmptyPlaylist(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	now := at(30 * time.Second)

	state, err := eng.Calculate(window, []domain.PlaylistItem{}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !state.Started {
		t.Error("expected Started=true, cycle has begun")
	}
	if state.Active {
		t.Error("expected Active=false for empty playlist")
	}
	if !state.PlaylistEmpty {
		t.Error("expected PlaylistEmpty=true")
	}
	if state.Media != nil {
		t.Error("expected nil Media for empty playlist — no fabricated blank")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 2 — Single item playlist; repeat within cycle
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_SingleItem(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	playlist := makePlaylist([2]int{1, 30}) // A = 30s

	cases := []struct {
		offset   time.Duration
		wantPos  int // 1-indexed playlist position
		wantElap int // elapsed within item (seconds)
	}{
		{0 * time.Second, 1, 0},
		{10 * time.Second, 1, 10},
		{29 * time.Second, 1, 29},
		{30 * time.Second, 1, 0},  // playlist repeats
		{59 * time.Second, 1, 29}, // still item 1
	}

	for _, tc := range cases {
		now := at(tc.offset)
		state, err := eng.Calculate(window, playlist, now)
		if err != nil {
			t.Fatalf("offset=%v: unexpected error: %v", tc.offset, err)
		}
		if !state.Active {
			t.Errorf("offset=%v: expected Active=true", tc.offset)
		}
		if state.Position != tc.wantPos {
			t.Errorf("offset=%v: Position=%d, want %d", tc.offset, state.Position, tc.wantPos)
		}
		if state.ElapsedSeconds != tc.wantElap {
			t.Errorf("offset=%v: ElapsedSeconds=%d, want %d", tc.offset, state.ElapsedSeconds, tc.wantElap)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 3 — Multiple items; position selection table
// ─────────────────────────────────────────────────────────────────────────────

// Playlist: A=30s, B=60s, C=120s  (total = 210s)
func TestEngine_MultipleItems(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	playlist := makePlaylist([2]int{1, 30}, [2]int{2, 60}, [2]int{3, 120})

	cases := []struct {
		offsetS int // seconds after epoch
		wantPos int // 1-indexed
	}{
		{0, 1},   // A [0,30)
		{29, 1},  // A
		{30, 2},  // B [30,90)
		{89, 2},  // B
		{90, 3},  // C [90,210)
		{209, 3}, // C
		{210, 1}, // A again (playlist repeats)
		{239, 1}, // A
		{240, 2}, // B
	}

	for _, tc := range cases {
		now := at(time.Duration(tc.offsetS) * time.Second)
		state, err := eng.Calculate(window, playlist, now)
		if err != nil {
			t.Fatalf("offset=%ds: %v", tc.offsetS, err)
		}
		if state.Position != tc.wantPos {
			t.Errorf("offset=%ds: Position=%d, want %d", tc.offsetS, state.Position, tc.wantPos)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 4 — Exact item boundaries [start, end) semantics
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_ExactBoundary(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	playlist := makePlaylist([2]int{1, 30}, [2]int{2, 60}) // A=30s, B=60s

	type tcase struct {
		label    string
		offsetNs time.Duration
		wantPos  int
	}

	cases := []tcase{
		// Item A owns [0ns, 30s)
		{"just before A/B boundary", 30*time.Second - time.Nanosecond, 1},
		// Exactly at 30s → item B
		{"exactly at A/B boundary", 30 * time.Second, 2},
		// just before B/playlist-end boundary
		{"just before playlist end", 90*time.Second - time.Nanosecond, 2},
		// Exactly at 90s → A again (playlist wraps)
		{"exactly at playlist end (wrap)", 90 * time.Second, 1},
	}

	for _, tc := range cases {
		now := at(tc.offsetNs)
		state, err := eng.Calculate(window, playlist, now)
		if err != nil {
			t.Fatalf("%s: %v", tc.label, err)
		}
		if state.Position != tc.wantPos {
			t.Errorf("%s: Position=%d, want %d", tc.label, state.Position, tc.wantPos)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 5 — Exact cycle boundary (18 000 s default)
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_ExactCycleBoundary(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(18000)
	// A=30s, total=30s. Repeats 600× inside a 5-h cycle.
	playlist := makePlaylist([2]int{1, 30})

	cases := []struct {
		label   string
		offsetS int
		wantPos int
		wantCyc int // expected cycle_position_seconds
	}{
		{"last second of cycle", 17999, 1, 17999},
		{"exact cycle boundary (18000s) → first item", 18000, 1, 0},
		{"double cycle boundary (36000s) → first item", 36000, 1, 0},
	}

	for _, tc := range cases {
		now := at(time.Duration(tc.offsetS) * time.Second)
		state, err := eng.Calculate(window, playlist, now)
		if err != nil {
			t.Fatalf("%s: %v", tc.label, err)
		}
		if state.Position != tc.wantPos {
			t.Errorf("%s: Position=%d, want %d", tc.label, state.Position, tc.wantPos)
		}
		if state.CyclePositionSeconds != tc.wantCyc {
			t.Errorf("%s: CyclePositionSeconds=%d, want %d", tc.label, state.CyclePositionSeconds, tc.wantCyc)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 6 — Playlist shorter than cycle (playlist repeats multiple times)
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_PlaylistShorterThanCycle(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(18000)                            // 5-hour cycle
	playlist := makePlaylist([2]int{1, 30}, [2]int{2, 60}) // total = 90s

	// At cycle position 0, 90, 180 … we should always land on position 1.
	for _, repIdx := range []int{0, 1, 2, 100} {
		offsetS := repIdx * 90
		now := at(time.Duration(offsetS) * time.Second)
		state, err := eng.Calculate(window, playlist, now)
		if err != nil {
			t.Fatalf("repIdx=%d: %v", repIdx, err)
		}
		if state.Position != 1 {
			t.Errorf("repIdx=%d (offset=%ds): Position=%d, want 1", repIdx, offsetS, state.Position)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 7 — Playlist longer than cycle (cycle boundary wins)
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_PlaylistLongerThanCycle(t *testing.T) {
	eng := service.NewPlaybackEngine()
	// Tiny cycle of 10 seconds for simplicity.
	window := makeWindow(10)
	// Playlist total = 100s (> 10s cycle).
	// Items: A=50s, B=50s
	playlist := makePlaylist([2]int{1, 50}, [2]int{2, 50})

	// Cycle resets at 10s, 20s, 30s…
	// At 0s: cyclePos=0 → playlistPos = 0 % 100 = 0 → A
	// At 10s: cyclePos=0 (reset) → A again
	for _, repIdx := range []int{0, 1, 2, 5} {
		offsetS := repIdx * 10
		now := at(time.Duration(offsetS) * time.Second)
		state, err := eng.Calculate(window, playlist, now)
		if err != nil {
			t.Fatalf("repIdx=%d: %v", repIdx, err)
		}
		if state.Position != 1 {
			t.Errorf("repIdx=%d (offset=%ds): Position=%d, want 1 (cycle boundary wins)", repIdx, offsetS, state.Position)
		}
		if state.CyclePositionSeconds != 0 {
			t.Errorf("repIdx=%d: CyclePositionSeconds=%d, want 0", repIdx, state.CyclePositionSeconds)
		}
	}

	// At offset 5s (halfway through 10s cycle), cyclePos=5 → playlistPos=5 → A (covers [0,50))
	state, err := eng.Calculate(window, playlist, at(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if state.Position != 1 {
		t.Errorf("offset=5s: Position=%d, want 1", state.Position)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 8 — Future epoch (cycle not started)
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_FutureEpoch(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(18000)
	playlist := makePlaylist([2]int{1, 30})

	// now is 1 hour BEFORE the epoch
	now := epoch.Add(-1 * time.Hour)

	state, err := eng.Calculate(window, playlist, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Started {
		t.Error("expected Started=false (now < epoch)")
	}
	if state.Active {
		t.Error("expected Active=false (not started)")
	}
	if state.Media != nil {
		t.Error("expected no Media for not-started state")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 9 — Invalid cycle duration
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_InvalidCycleDuration(t *testing.T) {
	eng := service.NewPlaybackEngine()
	playlist := makePlaylist([2]int{1, 30})

	for _, badCycle := range []int{0, -1, -18000} {
		window := makeWindow(badCycle)
		// makeWindow guards against 0 but we override directly for the test.
		window.CycleDurationSeconds = badCycle

		_, err := eng.Calculate(window, playlist, at(10*time.Second))
		if err == nil {
			t.Errorf("cycle=%d: expected error, got nil", badCycle)
			continue
		}
		if !errors.Is(err, service.ErrInvalidCycleDuration) {
			t.Errorf("cycle=%d: expected ErrInvalidCycleDuration, got %v", badCycle, err)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 10 — Invalid playlist duration (zero or negative)
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_InvalidPlaylistDuration(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)

	badPlaylists := [][]domain.PlaylistItem{
		// Zero duration item
		{
			{ID: 1, WindowID: 1, MediaItemID: 1, Position: 1, DurationSeconds: 0,
				IsActive: true, Media: makeMedia(1)},
		},
		// Negative duration item
		{
			{ID: 1, WindowID: 1, MediaItemID: 1, Position: 1, DurationSeconds: -5,
				IsActive: true, Media: makeMedia(1)},
		},
	}

	for i, playlist := range badPlaylists {
		_, err := eng.Calculate(window, playlist, at(10*time.Second))
		if err == nil {
			t.Errorf("badPlaylist[%d]: expected error, got nil", i)
			continue
		}
		if !errors.Is(err, service.ErrInvalidPlaylistDuration) {
			t.Errorf("badPlaylist[%d]: expected ErrInvalidPlaylistDuration, got %v", i, err)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 11 — Nil media pointer
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_NilMedia(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)

	playlist := []domain.PlaylistItem{
		{ID: 1, WindowID: 1, MediaItemID: 1, Position: 1, DurationSeconds: 30,
			IsActive: true, Media: nil}, // nil Media
	}

	_, err := eng.Calculate(window, playlist, at(10*time.Second))
	if err == nil {
		t.Fatal("expected error for nil Media, got nil")
	}
	if !errors.Is(err, service.ErrNilMedia) {
		t.Errorf("expected ErrNilMedia, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 12 — Unordered positions → rejected
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_UnsortedPlaylist(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)

	// Positions: 1, 3, 2 — not strictly ascending.
	playlist := []domain.PlaylistItem{
		{ID: 1, WindowID: 1, MediaItemID: 1, Position: 1, DurationSeconds: 30, IsActive: true, Media: makeMedia(1)},
		{ID: 2, WindowID: 1, MediaItemID: 2, Position: 3, DurationSeconds: 60, IsActive: true, Media: makeMedia(2)},
		{ID: 3, WindowID: 1, MediaItemID: 3, Position: 2, DurationSeconds: 30, IsActive: true, Media: makeMedia(3)},
	}

	_, err := eng.Calculate(window, playlist, at(10*time.Second))
	if err == nil {
		t.Fatal("expected error for unsorted playlist, got nil")
	}
	if !errors.Is(err, service.ErrUnsortedPlaylist) {
		t.Errorf("expected ErrUnsortedPlaylist, got %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 13 — Timezone equivalence
// UTC and non-UTC timestamps referring to the same instant must produce the
// same playback state.
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_TimezoneEquivalence(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	playlist := makePlaylist([2]int{1, 30}, [2]int{2, 60}, [2]int{3, 120})

	// Pick an instant: 75 seconds after epoch → should be in item B (position 2).
	nowUTC := epoch.Add(75 * time.Second)

	// Express the same instant in IST (+5:30)
	ist := time.FixedZone("IST", 5*3600+30*60)
	nowIST := nowUTC.In(ist)

	stateUTC, err := eng.Calculate(window, playlist, nowUTC)
	if err != nil {
		t.Fatalf("UTC: %v", err)
	}
	stateIST, err := eng.Calculate(window, playlist, nowIST)
	if err != nil {
		t.Fatalf("IST: %v", err)
	}

	if stateUTC.Position != stateIST.Position {
		t.Errorf("Position mismatch: UTC=%d IST=%d", stateUTC.Position, stateIST.Position)
	}
	if stateUTC.CyclePositionSeconds != stateIST.CyclePositionSeconds {
		t.Errorf("CyclePositionSeconds mismatch: UTC=%d IST=%d",
			stateUTC.CyclePositionSeconds, stateIST.CyclePositionSeconds)
	}
	if stateUTC.ElapsedSeconds != stateIST.ElapsedSeconds {
		t.Errorf("ElapsedSeconds mismatch: UTC=%d IST=%d",
			stateUTC.ElapsedSeconds, stateIST.ElapsedSeconds)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 14 — Large elapsed time (many days/cycles after epoch)
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_LargeElapsedTime(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(18000) // 5-hour cycle
	// A=30s playlist, repeats 600× per cycle.
	playlist := makePlaylist([2]int{1, 30})

	// 365 days after epoch — modulo arithmetic must still be correct.
	days365 := time.Duration(365*24) * time.Hour
	now := epoch.Add(days365)

	state, err := eng.Calculate(window, playlist, now)
	if err != nil {
		t.Fatalf("365 days later: %v", err)
	}
	if !state.Active {
		t.Error("expected Active=true")
	}
	if !state.Started {
		t.Error("expected Started=true")
	}
	// The only item in the playlist must be selected.
	if state.Position != 1 {
		t.Errorf("Position=%d, want 1", state.Position)
	}
	// Verify cycle_position is in [0, 18000).
	if state.CyclePositionSeconds < 0 || state.CyclePositionSeconds >= 18000 {
		t.Errorf("CyclePositionSeconds=%d is out of range [0, 18000)", state.CyclePositionSeconds)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 15 — Nanosecond boundary precision
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_NanosecondBoundary(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	playlist := makePlaylist([2]int{1, 30}, [2]int{2, 60}) // A=30s, B=60s

	// 1ns before the A→B boundary: should still be A.
	justBeforeB := 30*time.Second - time.Nanosecond
	state, err := eng.Calculate(window, playlist, at(justBeforeB))
	if err != nil {
		t.Fatal(err)
	}
	if state.Position != 1 {
		t.Errorf("1ns before boundary: Position=%d, want 1", state.Position)
	}

	// Exactly at the boundary: should be B.
	exactlyAtB := 30 * time.Second
	state, err = eng.Calculate(window, playlist, at(exactlyAtB))
	if err != nil {
		t.Fatal(err)
	}
	if state.Position != 2 {
		t.Errorf("exactly at boundary: Position=%d, want 2", state.Position)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 16 — Determinism / idempotency
// Running Calculate multiple times with the same inputs must give identical
// results (no global mutable state).
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_Determinism(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	playlist := makePlaylist([2]int{1, 30}, [2]int{2, 60}, [2]int{3, 120})
	now := at(75 * time.Second)

	const runs = 100
	first, err := eng.Calculate(window, playlist, now)
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i < runs; i++ {
		state, err := eng.Calculate(window, playlist, now)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if state.Position != first.Position {
			t.Errorf("run %d: Position=%d, want %d", i, state.Position, first.Position)
		}
		if state.ElapsedSeconds != first.ElapsedSeconds {
			t.Errorf("run %d: ElapsedSeconds=%d, want %d", i, state.ElapsedSeconds, first.ElapsedSeconds)
		}
		if state.RemainingSeconds != first.RemainingSeconds {
			t.Errorf("run %d: RemainingSeconds=%d, want %d", i, state.RemainingSeconds, first.RemainingSeconds)
		}
		if state.CyclePositionSeconds != first.CyclePositionSeconds {
			t.Errorf("run %d: CyclePositionSeconds=%d, want %d", i, state.CyclePositionSeconds, first.CyclePositionSeconds)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 17 — TotalPlaylistDuration is correct
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_TotalPlaylistDuration(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	playlist := makePlaylist([2]int{1, 30}, [2]int{2, 60}, [2]int{3, 120})
	want := 30 + 60 + 120 // 210

	state, err := eng.Calculate(window, playlist, at(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if state.TotalPlaylistDuration != want {
		t.Errorf("TotalPlaylistDuration=%d, want %d", state.TotalPlaylistDuration, want)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 18 — CalculatedAt is UTC and matches the `now` argument
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_CalculatedAtUTC(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	playlist := makePlaylist([2]int{1, 30})

	// Use a non-UTC timezone for `now`.
	ist := time.FixedZone("IST", 5*3600+30*60)
	now := epoch.Add(10 * time.Second).In(ist)

	state, err := eng.Calculate(window, playlist, now)
	if err != nil {
		t.Fatal(err)
	}

	if state.CalculatedAt.Location() != time.UTC {
		t.Errorf("CalculatedAt timezone=%v, want UTC", state.CalculatedAt.Location())
	}
	if !state.CalculatedAt.Equal(now.UTC()) {
		t.Errorf("CalculatedAt=%v, want %v", state.CalculatedAt, now.UTC())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 19 — ElapsedSeconds + RemainingSeconds ≈ item duration
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_ElapsedPlusRemainingEqualsItemDuration(t *testing.T) {
	eng := service.NewPlaybackEngine()
	window := makeWindow(0)
	playlist := makePlaylist([2]int{1, 30}, [2]int{2, 60}, [2]int{3, 120})

	offsets := []time.Duration{0, 5, 15, 29, 30, 60, 89, 90, 150}
	for _, offset := range offsets {
		state, err := eng.Calculate(window, playlist, at(offset*time.Second))
		if err != nil {
			t.Fatalf("offset=%v: %v", offset, err)
		}
		if !state.Active {
			continue
		}
		itemDur := playlist[state.Position-1].DurationSeconds
		// elapsed + remaining should equal item duration (within 1s due to integer flooring)
		sum := state.ElapsedSeconds + state.RemainingSeconds
		if sum < itemDur || sum > itemDur+1 {
			t.Errorf("offset=%vs: elapsed(%d)+remaining(%d)=%d, item_duration=%d",
				offset, state.ElapsedSeconds, state.RemainingSeconds, sum, itemDur)
		}
	}
}
