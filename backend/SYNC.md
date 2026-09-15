# Synchronized Playback Override Engine

> **Core Principle**:
> **“Sync is a temporary global playback override and does not modify the underlying playlists.”**

---

## 1. Overview & Architecture

The **Synchronized Playback Override Engine** enables an operator to broadcast an immediate, temporary playback override across **all display windows simultaneously**.

```
Operator Trigger (POST /api/sync)
         │
         ▼
┌────────────────────────────────────────────────────────┐
│             SyncService (internal/service)             │
│  - Validates media existence and duration              │
│  - PostgreSQL Advisory Transaction Lock (84848484)     │
│  - Atomically creates SyncEvent record in PostgreSQL   │
│  - Publishes SYNC_STARTED to WebSocket Hub            │
└────────┬───────────────────────────────────────────────┘
         │
         ├─────────────────────────────────────────┐
         ▼                                         ▼
┌────────────────────────────────┐    ┌─────────────────────────────────┐
│     WebSocket Hub (ws/hub.go)  │    │  PlaybackService (service)      │
│  - Broadcasts SYNC_STARTED to  │    │  - Checks GetActiveAt(now)      │
│    ALL connected windows       │    │  - When active: mode = "sync"   │
│  - When sync expires/canceled: │    │  - When expired: mode = "normal"│
│    Broadcasts SYNC_ENDED       │    │  - Deterministic wall-clock     │
│                                │    │    resumption via PlaybackEngine│
└────────────────────────────────┘    └─────────────────────────────────┘
```

---

## 2. Wall-Clock Resumption Semantics

When synchronization ends (either by natural expiration or operator cancellation), every window returns to its normal playlist playback using **current wall-clock time**.

- **Normal playlist is never modified, reordered, destroyed, or paused.**
- Playback position is computed as:
  $$\text{elapsed} = \text{now.UTC()} - \text{window.epoch\_start\_time.UTC()}$$
  $$\text{cyclePos} = \text{elapsed} \pmod{\text{cycle\_duration\_seconds}}$$
  $$\text{playlistPos} = \text{cyclePos} \pmod{\text{total\_playlist\_duration}}$$
- Each window resumes at the exact item it would have been playing had the sync never occurred.
- Any playlist additions, deletions, or reorderings performed during the sync are immediately reflected upon sync conclusion.

---

## 3. REST API Endpoints

### 3.1. Trigger Synchronization
**`POST /api/sync`**

#### Request Body
```json
{
  "media_id": 5,
  "duration_seconds": 30,
  "triggered_by": "operator"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `media_id` | integer | Yes | Positive integer referencing an existing media item. |
| `duration_seconds` | integer | Yes | Duration of override in seconds (1 to 18,000). |
| `triggered_by` | string | No | Identifier for auditing (defaults to `"operator"`). |

#### Response: `201 Created`
```json
{
  "success": true,
  "data": {
    "id": 12,
    "media_item_id": 5,
    "duration_seconds": 30,
    "started_at": "2026-09-15T10:00:00Z",
    "ends_at": "2026-09-15T10:00:30Z",
    "triggered_by": "operator",
    "status": "active",
    "created_at": "2026-09-15T10:00:00Z",
    "media": {
      "id": 5,
      "title": "Emergency Alert",
      "type": "image",
      "url": "https://cdn.example.com/alert.png",
      "default_duration_seconds": 30
    }
  }
}
```

#### Error Responses
- **`400 Bad Request`**:
  - `VALIDATION_ERROR`: Invalid JSON or `media_id <= 0`.
  - `INVALID_SYNC_DURATION`: `duration_seconds <= 0`.
  - `INVALID_MEDIA_TYPE`: Media item has unsupported type.
- **`404 Not Found`**:
  - `MEDIA_NOT_FOUND`: Specified `media_id` does not exist in database.
- **`409 Conflict`**:
  - `SYNC_ALREADY_ACTIVE`: An active, unexpired sync event is already running.

---

### 3.2. Query Active Sync State
**`GET /api/sync/state`** (and alias **`GET /api/sync/status`**)

#### Response When Active: `200 OK`
```json
{
  "active": true,
  "sync_id": 12,
  "media": {
    "id": 5,
    "title": "Emergency Alert",
    "type": "image",
    "url": "https://cdn.example.com/alert.png"
  },
  "started_at": "2026-09-15T10:00:00Z",
  "ends_at": "2026-09-15T10:00:30Z",
  "duration_seconds": 30,
  "remaining_seconds": 18,
  "server_time": "2026-09-15T10:00:12Z"
}
```

#### Response When Inactive: `200 OK`
```json
{
  "active": false
}
```

---

### 3.3. Cancel Active Sync
**`POST /api/sync/cancel`**

Prematurely terminates the active sync event, returning all windows to normal playback immediately.

#### Response: `200 OK`
```json
{
  "success": true,
  "message": "Sync playback cancelled successfully",
  "data": {
    "id": 12,
    "status": "cancelled"
  }
}
```

#### Error Response
- **`409 Conflict`**:
  - `SYNC_NOT_ACTIVE`: No synchronized playback event is currently active.

---

### 3.4. Window Effective Playback Integration
**`GET /api/windows/:id`**

During an active sync event:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "name": "Window 1",
    "playlist": [ ... ],
    "current_playback_state": {
      "mode": "sync",
      "sync_event_id": 12,
      "started": true,
      "active": true,
      "playlist_empty": false,
      "window_id": 1,
      "media": {
        "id": 5,
        "title": "Emergency Alert",
        "type": "image",
        "url": "https://cdn.example.com/alert.png"
      },
      "elapsed_seconds": 12,
      "remaining_seconds": 18,
      "calculated_at": "2026-09-15T10:00:12Z"
    }
  }
}
```

After sync expiration/cancellation:
```json
{
  "current_playback_state": {
    "mode": "normal",
    "playlist_item_id": 102,
    "position": 2,
    "elapsed_seconds": 40,
    "remaining_seconds": 20
  }
}
```

---

## 4. WebSocket Event Delivery

### 4.1. Global `SYNC_STARTED` Event
Broadcast to **ALL** connected display clients regardless of window subscription:
```json
{
  "type": "SYNC_STARTED",
  "server_time": "2026-09-15T10:00:00Z",
  "timestamp": "2026-09-15T10:00:00Z",
  "sync": {
    "event_id": 12,
    "media": {
      "id": 5,
      "title": "Emergency Alert",
      "type": "image",
      "url": "https://cdn.example.com/alert.png"
    },
    "started_at": "2026-09-15T10:00:00Z",
    "ends_at": "2026-09-15T10:00:30Z",
    "duration_seconds": 30
  }
}
```

### 4.2. Global `SYNC_ENDED` Event
Broadcast to **ALL** connected display clients when sync completes or is cancelled:
```json
{
  "type": "SYNC_ENDED",
  "server_time": "2026-09-15T10:00:30Z",
  "timestamp": "2026-09-15T10:00:30Z",
  "sync": {
    "event_id": 12,
    "reason": "completed"
  }
}
```

### 4.3. Client Connection During Sync
If a new client connects or re-subscribes while sync is active, its initial `STATE_SNAPSHOT` reflects `mode: "sync"` with active sync media and remaining duration.

---

## 5. Concurrency & Overlap Protection

1. **Transaction-Level Advisory Lock (`pg_try_advisory_xact_lock(84848484)`)**:
   - Ensures strict mutual exclusion across all backend instances.
2. **Atomic Verification (`CreateIfNoActive`)**:
   - Checks if any record in `sync_events` has `status = 'active' AND ends_at > started_at`.
   - If found, immediately returns `SYNC_ALREADY_ACTIVE` (`409 Conflict`) without modifying state.
   - Automatically marks stale expired events as `'completed'`.

---

## 6. Server Crash & Restart Recovery

- **Wall-Clock Authoritative**: Active sync state is determined by whether $\text{now} < \text{ends\_at}$, not by volatile in-memory timers.
- **On Boot (`Recover`)**:
  - Unexpired active events continue running until `ends_at`.
  - Expired events are reconciled to `'completed'`.
