# EVA Bharat Media Sequencer - REST API Reference

Compose base URL: `http://localhost:3000/api` (proxied to the private backend). Direct local-development base URL: `http://localhost:8080/api`.

All responses use a consistent JSON envelope:
- Success: `{ "success": true, "data": { ... } }`
- Error:   `{ "success": false, "error": { "code": "...", "message": "..." } }`

---

## Health

### GET /api/health
Returns server health, database connectivity, and current UTC timestamp.

Response 200 OK (database connected):
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "service": "eva-media-sequencer",
    "environment": "development",
    "database": "connected",
    "timestamp": "2026-09-15T00:00:00.000Z"
  }
}
```

Response 503 (database unreachable):
```json
{ "success": false, "data": { "status": "degraded", "database": "unreachable" } }
```

`database` values: `"connected"` | `"unreachable"` | `"unconfigured"`

---

## Windows

### GET /api/windows
Returns all display windows.

### POST /api/windows
Creates a new display window.

Request body:
```json
{
  "name": "Lobby Display",
  "description": "Optional description",
  "cycle_duration_seconds": 18000
}
```

Field constraints:
- `name`: required, non-empty, unique (max 100 chars)
- `description`: optional
- `cycle_duration_seconds`: optional, default 18000, must be > 0

Response 201: Created window object.

Errors: `VALIDATION_ERROR` 400

### GET /api/windows/:id
Returns a single window with its complete ordered playlist and current deterministic playback state.

Response 200:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "name": "Lobby Display",
    "cycle_duration_seconds": 18000,
    "epoch_start_time": "2026-09-15T00:00:00Z",
    "playlist": [],
    "current_playback_state": {
      "mode": "normal",
      "started": true,
      "active": true,
      "playlist_empty": false,
      "window_id": 1,
      "playlist_item_id": 2,
      "position": 1,
      "media": {},
      "elapsed_seconds": 15,
      "remaining_seconds": 45,
      "cycle_position_seconds": 135,
      "total_playlist_duration": 110,
      "calculated_at": "2026-09-15T00:02:15Z"
    }
  }
}
```

`mode` values: `"normal"` | `"sync"` | `"inactive"`

Errors: `WINDOW_NOT_FOUND` 404

### PATCH /api/windows/:id
Partially updates a display window. All fields optional:
```json
{ "name": "New Name", "description": "...", "cycle_duration_seconds": 7200 }
```

Errors: `VALIDATION_ERROR` 400, `WINDOW_NOT_FOUND` 404

### DELETE /api/windows/:id
Deletes a display window and cascades to remove all its playlist items.

Response: `{ "success": true, "message": "Window deleted successfully" }`
Errors: `WINDOW_NOT_FOUND` 404

---

## Media Library

### GET /api/media
Returns all registered media assets.

### POST /api/media
Registers a new media asset.

Request body:
```json
{
  "title": "My Clip",
  "type": "video",
  "url": "https://example.com/clip.mp4",
  "default_duration_seconds": 30
}
```

Field constraints:
- `title`: required (max 255 chars)
- `type`: required, one of `"image"` | `"video"` | `"blank"`
- `url`: required
- `default_duration_seconds`: required, must be > 0

Response 201: Created media object.
Errors: `VALIDATION_ERROR` 400, `INVALID_MEDIA_TYPE` 400, `INVALID_DURATION` 400

### GET /api/media/:id
Returns a single media asset.
Errors: `MEDIA_NOT_FOUND` 404

### PATCH /api/media/:id
Partially updates a media asset. All fields optional.
Errors: `MEDIA_NOT_FOUND` 404, `VALIDATION_ERROR` 400

### DELETE /api/media/:id
Deletes a media asset. Returns 409 if referenced by a playlist.
Errors: `MEDIA_NOT_FOUND` 404, `MEDIA_IN_USE` 409

---

## Playlists

### GET /api/windows/:id/playlist
Returns the ordered playlist for the specified window (position ASC, joined with media).

### POST /api/windows/:id/playlist
Appends a media item to the end of the window's playlist.

Request body:
```json
{ "media_id": 3, "duration_seconds": 15 }
```

- `media_id`: required
- `duration_seconds`: optional, defaults to media's `default_duration_seconds`

Response 201: Created playlist item.
WebSocket: Broadcasts `PLAYLIST_UPDATED` to window subscribers.
Errors: `WINDOW_NOT_FOUND` 404, `MEDIA_NOT_FOUND` 404, `VALIDATION_ERROR` 400, `PLAYLIST_DURATION_EXCEEDED` 400

The active playlist total cannot exceed the window's cycle duration. This prevents trailing items from becoming unreachable.

### DELETE /api/windows/:id/playlist/:itemId
Removes a playlist item and compacts positions.

Response: `{ "success": true, "message": "Playlist item removed successfully" }`
WebSocket: Broadcasts `PLAYLIST_UPDATED` to window subscribers.
Errors: `WINDOW_NOT_FOUND` 404, `PLAYLIST_ITEM_NOT_FOUND` 404

### PATCH /api/windows/:id/playlist/reorder
Reorders the entire playlist. Must include ALL existing item IDs for the window.

Request body:
```json
{ "item_ids": [5, 2, 8, 1] }
```

Response 200: Complete reordered playlist.
WebSocket: Broadcasts `PLAYLIST_UPDATED` to window subscribers.
Errors: `INVALID_REORDER` 400, `WINDOW_NOT_FOUND` 404

---

## Synchronized Playback Override

### POST /api/sync
Initiates a global synchronized playback override across ALL display windows.

Request body:
```json
{
  "media_id": 5,
  "duration_seconds": 120,
  "triggered_by": "operator"
}
```

- `media_id`: required (must exist, type: image/video/blank)
- `duration_seconds`: required, from 1 to 18,000 seconds
- `triggered_by`: optional, default "operator"

Response 201:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "media_item_id": 5,
    "duration_seconds": 120,
    "started_at": "2026-09-15T00:00:00Z",
    "ends_at": "2026-09-15T00:02:00Z",
    "triggered_by": "operator",
    "status": "active"
  }
}
```

WebSocket: Broadcasts `SYNC_STARTED` to all connected clients.
Errors: `SYNC_ALREADY_ACTIVE` 409, `MEDIA_NOT_FOUND` 404, `INVALID_SYNC_DURATION` 400

### GET /api/sync/state  (also: GET /api/sync/status)
Returns the currently active global sync state.

Response 200 (active):
```json
{
  "success": true,
  "active": true,
  "sync_id": 1,
  "media": {},
  "started_at": "...",
  "ends_at": "...",
  "duration_seconds": 120,
  "remaining_seconds": 87,
  "server_time": "..."
}
```

Response 200 (inactive): `{ "success": true, "active": false }`

### POST /api/sync/cancel
Prematurely cancels the currently active global sync playback event.

Response 200:
```json
{ "success": true, "message": "Sync playback cancelled successfully", "data": {} }
```

WebSocket: Broadcasts `SYNC_ENDED` to all connected clients.
Errors: `SYNC_NOT_ACTIVE` 409

---

## WebSocket

### GET /api/ws
Upgrades to a WebSocket connection. See WEBSOCKET.md for full protocol documentation.

After connecting, send: `{ "type": "SUBSCRIBE", "window_id": 1 }`

You will receive:
- `STATE_SNAPSHOT` on subscription
- `PLAYLIST_UPDATED` on playlist mutations (window-specific)
- `SYNC_STARTED` / `SYNC_ENDED` global broadcasts
- `TIME_SYNC` periodic server UTC reference

---

## Error Codes Reference

| Code                    | HTTP | Description                                      |
|-------------------------|------|--------------------------------------------------|
| VALIDATION_ERROR        | 400  | Invalid request body or parameters               |
| INVALID_MEDIA_TYPE      | 400  | Unsupported media type                           |
| INVALID_DURATION        | 400  | Duration must be > 0                             |
| INVALID_REORDER         | 400  | Reorder list is incomplete or invalid            |
| INVALID_SYNC_DURATION   | 400  | Sync duration must be > 0                        |
| WINDOW_NOT_FOUND        | 404  | Window does not exist                            |
| MEDIA_NOT_FOUND         | 404  | Media item does not exist                        |
| PLAYLIST_ITEM_NOT_FOUND | 404  | Playlist item does not exist                     |
| SYNC_NOT_FOUND          | 404  | Sync event does not exist                        |
| MEDIA_IN_USE            | 409  | Media referenced by playlist; cannot delete      |
| SYNC_ALREADY_ACTIVE     | 409  | A sync event is already running                  |
| SYNC_NOT_ACTIVE         | 409  | No active sync event to cancel                   |
| INTERNAL_ERROR          | 500  | Unexpected server error (safe message; logged)   |
