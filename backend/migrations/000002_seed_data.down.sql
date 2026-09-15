-- ==============================================================================
-- Migration: 000002_seed_data.down.sql
-- Description: Reversible rollback of development seed data
-- ==============================================================================

-- Remove seeded playlist items
DELETE FROM playlist_items WHERE id IN (1, 2, 3, 4, 5, 6, 7, 8);

-- Remove seeded windows (cascades any remaining playlist items)
DELETE FROM windows WHERE id IN (1, 2, 3);

-- Remove seeded media items
DELETE FROM media_items WHERE id IN (1, 2, 3, 4, 5, 6, 7, 8);
