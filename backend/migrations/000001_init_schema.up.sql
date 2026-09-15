-- ==============================================================================
-- Migration: 000001_init_schema.up.sql
-- Description: Initial schema for EVA Bharat Multi-Window Media Sequencer
-- Timezone Policy: All timestamps use TIMESTAMPTZ to ensure UTC accuracy for
--                  cross-window synchronization and 5-hour cycle epoch math.
-- ==============================================================================

-- 1. WINDOWS TABLE
-- Represents independent display screens/windows.
-- cycle_duration_seconds defaults to 18,000s (exactly 5 hours), but remains configurable.
CREATE TABLE IF NOT EXISTS windows (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    cycle_duration_seconds INT NOT NULL DEFAULT 18000 CHECK (cycle_duration_seconds > 0),
    epoch_start_time TIMESTAMPTZ NOT NULL DEFAULT '2026-01-01 00:00:00+00',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. MEDIA ITEMS TABLE
-- Global media asset library supporting images, videos, and configured blank screens.
CREATE TABLE IF NOT EXISTS media_items (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL CHECK (type IN ('image', 'video', 'blank')),
    url TEXT NOT NULL,
    default_duration_seconds INT NOT NULL CHECK (default_duration_seconds > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 3. PLAYLIST ITEMS TABLE
-- Ordered sequence of media assigned to each window.
-- Constraints:
--   - Cascade on window delete: deleting a window removes its playlist sequence.
--   - Restrict on media delete: prevents deleting a media asset actively configured in a playlist.
--   - Position must be >= 1.
--   - Duration must be > 0.
--   - Unique (window_id, position): strictly prevents duplicate sequence indices.
CREATE TABLE IF NOT EXISTS playlist_items (
    id SERIAL PRIMARY KEY,
    window_id INT NOT NULL REFERENCES windows(id) ON DELETE CASCADE,
    media_item_id INT NOT NULL REFERENCES media_items(id) ON DELETE RESTRICT,
    position INT NOT NULL CHECK (position >= 1),
    duration_seconds INT NOT NULL CHECK (duration_seconds > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_window_position UNIQUE (window_id, position)
);

-- 4. SYNC EVENTS TABLE
-- Audit and persistent history for synchronized override playback events.
CREATE TABLE IF NOT EXISTS sync_events (
    id SERIAL PRIMARY KEY,
    media_item_id INT NOT NULL REFERENCES media_items(id) ON DELETE RESTRICT,
    duration_seconds INT NOT NULL CHECK (duration_seconds > 0),
    started_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    triggered_by VARCHAR(100) NOT NULL DEFAULT 'admin',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 5. INDEXES FOR PERFORMANCE
-- Fast lookup by window and sequential position during playback resolution
CREATE INDEX IF NOT EXISTS idx_playlist_window_pos ON playlist_items(window_id, position);

-- Fast lookup for currently active or expiring sync events
CREATE INDEX IF NOT EXISTS idx_sync_events_active ON sync_events(status, ends_at);
