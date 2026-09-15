# EVA Bharat Media Sequencer

A full-stack control plane for deterministic media playback across independent display windows. Operators manage media and playlists from one interface and can temporarily override every display with a synchronized item. Normal playback is calculated from UTC wall-clock time, so a display can reconnect without storing a fragile local cursor.

## What is included

- Go/Gin REST API with PostgreSQL persistence and automatic migrations
- Deterministic per-window playback with configurable cycle duration
- Global synchronized playback with overlap protection and natural/cancelled end events
- WebSocket subscriptions, state snapshots, playlist updates, and server-time correction
- React/TypeScript operator console and display wall
- Docker images for the API and frontend, plus an isolated integration-test database
- Backend unit/integration tests, frontend unit tests, static checks, and CI

The implementation uses Go 1.26 with Gin and pgx, PostgreSQL 16, React 18, TypeScript, Vite, native browser WebSockets, Nginx, and Docker Compose.

## Run the complete application

Requirements: Docker Desktop with Compose.

```bash
docker compose up --build
```

Open the operator console at <http://localhost:3000>. Through the same origin, the API is available at <http://localhost:3000/api> and its health endpoint is <http://localhost:3000/api/health>. Database migrations and sample windows, media, and playlists are created automatically on first start. The backend is intentionally private to the Compose network, avoiding collisions with locally running services on port 8080.

Stop the services with `docker compose down`. The normal database is retained in the `postgres_data` volume.

## Local development

Requirements: Go 1.26.4+, Node.js 22+, and Docker.

```bash
docker compose up -d postgres

cd backend
# PowerShell; the Compose database is exposed on 5434 by default.
$env:DATABASE_URL='postgres://postgres:postgres@localhost:5434/eva_media?sslmode=disable'
go run ./cmd/server

# In another terminal
cd frontend
npm ci
npm run dev
```

The Vite server runs at <http://localhost:5173> and proxies `/api` to the backend. The frontend uses same-origin REST and WebSocket URLs by default, so production builds do not contain localhost endpoints.

## Tests and checks

Unit tests never use the development database. Database integration tests require an explicit URL whose database name contains `test`.

```bash
# Backend unit tests and static analysis
cd backend
go test -count=1 ./...
go vet ./...

# Frontend tests, type check, and production build
cd ../frontend
npm test
npm run check
npm run build
```

To include PostgreSQL integration tests:

```bash
docker compose --profile test up -d postgres_test

# PowerShell
$env:TEST_DATABASE_URL='postgres://postgres:postgres@localhost:55433/eva_media_test?sslmode=disable'
cd backend
go test -count=1 ./...
```

The test database is separate from the application database and uses temporary container storage.

With the complete stack running, execute the API and WebSocket acceptance flows from the repository root:

```bash
node scripts/test-playback.js
node scripts/test-playlist-manager.js
```

## Playback model

For a window with epoch `E`, cycle length `C`, and playlist total `P`, the server computes:

```text
cyclePosition    = (nowUTC - E) mod C
playlistPosition = cyclePosition mod P
```

The item whose cumulative duration contains `playlistPosition` is active. If the playlist does not fill its cycle, this implementation repeats the playlist within the cycle. A playlist cannot exceed its window cycle, preventing items that could never be reached. Video elements loop when their configured slot is longer than the source file.

A synchronized override does not pause or modify playlists. When it ends, each display immediately returns to the position implied by current UTC time. The backend emits `SYNC_ENDED` for both natural completion and operator cancellation.

## Architecture

```text
React operator console / display wall
             |
      REST + WebSocket
             |
Gin handlers -> services -> PostgreSQL repositories
                   |
        deterministic playback engine
                   |
             WebSocket hub
```

The single in-process WebSocket hub is deliberately simple and appropriate for one API instance. Horizontal scaling requires a shared pub/sub transport such as Redis or PostgreSQL `LISTEN/NOTIFY`.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | API listen port |
| `ENVIRONMENT` | `development` | Runtime environment label |
| `DATABASE_URL` | local PostgreSQL URL | PostgreSQL connection string |
| `POSTGRES_HOST_PORT` | `5434` | Optional host port for the Compose database |
| `CORS_ALLOWED_ORIGINS` | local frontend origins | Comma-separated browser origins; also enforced for WebSocket upgrades |
| `VITE_API_BASE_URL` | `/api` | Optional frontend REST override |
| `VITE_WS_URL` | derived `/api/ws` URL | Optional frontend WebSocket override |
| `TEST_DATABASE_URL` | none | Explicit isolated database used by integration tests |

For a public deployment, change all database credentials, set `CORS_ALLOWED_ORIGINS` to the exact HTTPS site origin, terminate TLS at a reverse proxy or load balancer, and put the operator routes behind your organization’s authentication layer. Do not expose the sample Compose credentials to the internet.

## Manual deployment from GitHub

The repository is ready for the hosting flow you requested: `render.yaml` configures the Go API, while `netlify.toml` configures the React build. Netlify does not use a YAML manifest.

1. Create or link a Supabase project, then run `npx supabase@latest link --project-ref <project-ref>` and `npx supabase@latest db push`. The Supabase migrations create the schema, deterministic demo data, and block direct anonymous table access.
2. In Render, create a Blueprint from this repository. Set `DATABASE_URL` to the Supabase direct or session-pooler PostgreSQL URI with TLS enabled, and set `CORS_ALLOWED_ORIGINS` to the final Netlify origin, for example `https://your-site.netlify.app`.
3. In Netlify, import this repository. The committed configuration builds from `frontend/`. Add `VITE_API_BASE_URL=https://your-render-service.onrender.com/api` and `VITE_WS_URL=wss://your-render-service.onrender.com/api/ws`, then deploy.
4. If the Netlify hostname changes, update `CORS_ALLOWED_ORIGINS` in Render and redeploy the API. Verify `/api/health`, every operator route, and sync playback in two browser tabs.

Keep the Supabase database password and all platform tokens in their hosting dashboards. They are never required in Git or in the frontend bundle.

## Project map

```text
backend/
  cmd/server/                 application entry point
  internal/api/              routing, handlers, middleware
  internal/service/          playback and sync rules
  internal/repository/       PostgreSQL access
  internal/ws/               WebSocket connections and broadcasts
  migrations/                schema and sample data
frontend/
  src/api/                    typed REST client
  src/components/             operator and playback UI
  src/hooks/                  playback and data orchestration
  src/websocket/              protocol and reconnect logic
docker-compose.yml            complete local stack and test DB profile
```

Detailed protocol references: [REST API](backend/API.md), [WebSocket](backend/WEBSOCKET.md), and [synchronized playback](backend/SYNC.md).

## Deliberate scope

This submission focuses on the assignment’s playback, persistence, WebSocket, and recovery requirements. The API supports setting a duration when a playlist item is inserted, but it intentionally has no endpoint for editing that duration afterward; remove and re-add the item to change it. Production identity management and multi-instance event distribution are deployment concerns rather than hidden demo credentials or extra framework code.
