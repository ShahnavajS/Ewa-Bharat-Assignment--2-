# EVA Bharat frontend

React and TypeScript operator console for the media sequencer. It provides the display wall, playlist/media administration, synchronized override controls, and live system status.

## Development

```bash
npm ci
npm run dev
```

Open <http://localhost:5173>. Vite proxies `/api` and `/api/ws` to the backend at `http://localhost:8080`.

## Verification

```bash
npm test       # Vitest unit tests
npm run check  # TypeScript only
npm run build  # TypeScript and optimized production bundle
```

## Runtime configuration

REST defaults to same-origin `/api`; WebSocket defaults to same-origin `/api/ws`. This works with the included Nginx container and avoids baking localhost into production assets. Define `VITE_API_BASE_URL` or `VITE_WS_URL` only when the two services intentionally use different origins.

The production image is built by `frontend/Dockerfile`. Nginx serves the single-page app, falls back to `index.html` for client navigation, and proxies API and WebSocket traffic to the backend service.
