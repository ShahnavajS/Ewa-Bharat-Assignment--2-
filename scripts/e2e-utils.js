import assert from 'node:assert/strict';

export { assert };

export const apiBase = (process.env.EVA_API_BASE_URL || 'http://localhost:3000/api').replace(/\/$/, '');

export async function api(path, options = {}) {
  const response = await fetch(`${apiBase}${path}`, {
    ...options,
    headers: { 'content-type': 'application/json', ...options.headers },
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(`${options.method || 'GET'} ${path}: ${response.status} ${JSON.stringify(body)}`);
  }
  return Object.hasOwn(body, 'data') ? body.data : body;
}

export async function playlist(windowId) {
  const value = await api(`/windows/${windowId}/playlist`);
  return Array.isArray(value) ? value : value.items;
}

export function websocketURL() {
  const url = new URL(apiBase);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  url.pathname = `${url.pathname.replace(/\/$/, '')}/ws`;
  return url.toString();
}

export async function subscribe(windowId) {
  const socket = new WebSocket(websocketURL());
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('WebSocket connection timed out')), 5000);
    socket.addEventListener('open', () => {
      clearTimeout(timer);
      socket.send(JSON.stringify({ type: 'SUBSCRIBE', window_id: windowId }));
      resolve();
    }, { once: true });
    socket.addEventListener('error', () => reject(new Error('WebSocket connection failed')), { once: true });
  });
  return socket;
}

export function waitForMessage(socket, predicate, timeoutMs = 5000) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      socket.removeEventListener('message', onMessage);
      reject(new Error('Expected WebSocket event was not received'));
    }, timeoutMs);
    const onMessage = (event) => {
      const message = JSON.parse(String(event.data));
      if (!predicate(message)) return;
      clearTimeout(timer);
      socket.removeEventListener('message', onMessage);
      resolve(message);
    };
    socket.addEventListener('message', onMessage);
  });
}

export const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
