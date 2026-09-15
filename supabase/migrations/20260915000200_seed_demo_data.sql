-- ==============================================================================
-- Migration: 000002_seed_data.up.sql
-- Description: Deterministic development seed data for EVA Bharat Media Sequencer
-- Windows: 3 display windows (Lobby, Reception, Conference)
-- Media Items: 8 assets (images, videos, 1 explicit blank asset)
-- Playlists: Configured sequences with diverse media types and durations
-- Idempotency: ON CONFLICT DO NOTHING + explicit sequence synchronization
-- ==============================================================================

-- 1. SEED DISPLAY WINDOWS
-- cycle_duration_seconds = 18,000s (5 hours)
-- epoch_start_time = 1 hour prior to execution time, ensuring immediate active playback
INSERT INTO windows (id, name, description, cycle_duration_seconds, epoch_start_time)
VALUES
    (
        1,
        'Lobby Display',
        'Main entrance foyer display screen for corporate visitor greetings and announcements',
        18000,
        CURRENT_TIMESTAMP - INTERVAL '1 hour'
    ),
    (
        2,
        'Reception Display',
        'Front-desk reception display screen for company introduction and brand showcase',
        18000,
        CURRENT_TIMESTAMP - INTERVAL '1 hour'
    ),
    (
        3,
        'Conference Display',
        'Executive conference hall display screen for event schedules, announcements, and intermission intervals',
        18000,
        CURRENT_TIMESTAMP - INTERVAL '1 hour'
    )
ON CONFLICT (id) DO NOTHING;

-- Synchronize windows primary key sequence
SELECT setval('windows_id_seq', (SELECT COALESCE(MAX(id), 1) FROM windows));

-- 2. SEED MEDIA ASSETS (Images, Videos, Blank)
INSERT INTO media_items (id, title, type, url, default_duration_seconds)
VALUES
    (
        1,
        'Welcome Image',
        'image',
        'https://images.unsplash.com/photo-1497366216548-37526070297c?auto=format&fit=crop&w=1920&q=80',
        30
    ),
    (
        2,
        'Promotion Video',
        'video',
        'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4',
        60
    ),
    (
        3,
        'Information Image',
        'image',
        'https://images.unsplash.com/photo-1497215728101-856f4ea42174?auto=format&fit=crop&w=1920&q=80',
        20
    ),
    (
        4,
        'Company Video',
        'video',
        'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ElephantsDream.mp4',
        45
    ),
    (
        5,
        'Schedule Image',
        'image',
        'https://images.unsplash.com/photo-1517502884422-41eaead166d4?auto=format&fit=crop&w=1920&q=80',
        20
    ),
    (
        6,
        'Announcement Video',
        'video',
        'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4',
        90
    ),
    (
        7,
        'Product Showcase Image',
        'image',
        'https://images.unsplash.com/photo-1460925895917-afdab827c52f?auto=format&fit=crop&w=1920&q=80',
        30
    ),
    (
        8,
        'Explicit Blank Screen',
        'blank',
        'about:blank',
        10
    )
ON CONFLICT (id) DO NOTHING;

-- Synchronize media_items primary key sequence
SELECT setval('media_items_id_seq', (SELECT COALESCE(MAX(id), 1) FROM media_items));

-- 3. SEED PLAYLIST ITEMS
-- Window 1 (Lobby Display): Total duration = 110s
--   - Pos 1: Welcome Image (30s)
--   - Pos 2: Promotion Video (60s)
--   - Pos 3: Information Image (20s)
INSERT INTO playlist_items (id, window_id, media_item_id, position, duration_seconds, is_active)
VALUES
    (1, 1, 1, 1, 30, true),
    (2, 1, 2, 2, 60, true),
    (3, 1, 3, 3, 20, true)
ON CONFLICT (id) DO NOTHING;

-- Window 2 (Reception Display): Total duration = 75s
--   - Pos 1: Company Video (45s)
--   - Pos 2: Welcome Image (30s)
INSERT INTO playlist_items (id, window_id, media_item_id, position, duration_seconds, is_active)
VALUES
    (4, 2, 4, 1, 45, true),
    (5, 2, 1, 2, 30, true)
ON CONFLICT (id) DO NOTHING;

-- Window 3 (Conference Display): Total duration = 120s
--   - Pos 1: Schedule Image (20s)
--   - Pos 2: Announcement Video (90s)
--   - Pos 3: Explicit Blank Screen (10s)
INSERT INTO playlist_items (id, window_id, media_item_id, position, duration_seconds, is_active)
VALUES
    (6, 3, 5, 1, 20, true),
    (7, 3, 6, 2, 90, true),
    (8, 3, 8, 3, 10, true)
ON CONFLICT (id) DO NOTHING;

-- Synchronize playlist_items primary key sequence
SELECT setval('playlist_items_id_seq', (SELECT COALESCE(MAX(id), 1) FROM playlist_items));
