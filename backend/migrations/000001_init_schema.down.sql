-- ==============================================================================
-- Migration: 000001_init_schema.down.sql
-- Description: Rollback initial schema in reverse dependency order
-- ==============================================================================

DROP INDEX IF EXISTS idx_sync_events_active;
DROP INDEX IF EXISTS idx_playlist_window_pos;

DROP TABLE IF EXISTS sync_events;
DROP TABLE IF EXISTS playlist_items;
DROP TABLE IF EXISTS media_items;
DROP TABLE IF EXISTS windows;
