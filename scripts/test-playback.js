import { api, assert, subscribe, waitForMessage } from './e2e-utils.js';

const sockets = [];
let temporaryWindowId;
let syncStarted = false;
let syncPlaylistItemId;
let syncWindowId;

try {
  const health = await api('/health');
  assert.equal(health.status, 'ok');
  assert.equal(health.database, 'connected');

  const windows = await api('/windows');
  const media = await api('/media');
  assert.ok(windows.length >= 3, 'expected at least three display windows');
  assert.ok(media.some((item) => item.type === 'blank'), 'expected an explicit blank media item');

  const details = await Promise.all(windows.slice(0, 3).map((window) => api(`/windows/${window.id}`)));
  details.forEach((window) => {
    assert.equal(window.cycle_duration_seconds, 18000);
    assert.ok(window.playlist.length > 0, `window ${window.id} should have a seeded playlist`);
    assert.equal(window.current_playback_state.mode, 'normal');
  });
  assert.equal(new Set(details.map((window) => window.playlist.map((item) => item.id).join(','))).size, 3);

  const emptyWindow = await api('/windows', {
    method: 'POST',
    body: JSON.stringify({ name: `E2E Empty ${Date.now()}`, cycle_duration_seconds: 18000 }),
  });
  temporaryWindowId = emptyWindow.id;
  const emptyDetails = await api(`/windows/${emptyWindow.id}`);
  assert.equal(emptyDetails.current_playback_state.mode, 'inactive');
  assert.equal(emptyDetails.current_playback_state.playlist_empty, true);

  sockets.push(await subscribe(windows[0].id), await subscribe(windows[1].id));
  const snapshotEvents = sockets.map((socket, index) => {
    const event = waitForMessage(socket, (message) => message.type === 'STATE_SNAPSHOT');
    socket.send(JSON.stringify({ type: 'SUBSCRIBE', window_id: windows[index].id }));
    return event;
  });
  const snapshots = await Promise.all(snapshotEvents);
  snapshots.forEach((snapshot, index) => {
    assert.equal(snapshot.window_id, windows[index].id);
    assert.equal(snapshot.playback.current_playback_state.mode, 'normal');
  });

  const startedEvents = sockets.map((socket) => waitForMessage(socket, (message) => message.type === 'SYNC_STARTED'));
  const syncMedia = media.find((item) => item.type === 'image') || media[0];
  await api('/sync', {
    method: 'POST',
    body: JSON.stringify({ media_id: syncMedia.id, duration_seconds: 10, triggered_by: 'local-e2e' }),
  });
  syncStarted = true;
  await Promise.all(startedEvents);

  const synced = await Promise.all(windows.slice(0, 3).map((window) => api(`/windows/${window.id}`)));
  synced.forEach((window) => {
    assert.equal(window.current_playback_state.mode, 'sync');
    assert.equal(window.current_playback_state.media.id, syncMedia.id);
  });

  syncWindowId = windows[0].id;
  const unaffectedBefore = details[1].playlist.map((item) => item.id);
  const playlistEvent = waitForMessage(
    sockets[0],
    (message) => message.type === 'PLAYLIST_UPDATED' && message.window_id === syncWindowId,
  );
  const addedDuringSync = await api(`/windows/${syncWindowId}/playlist`, {
    method: 'POST',
    body: JSON.stringify({ media_id: syncMedia.id, duration_seconds: 5 }),
  });
  syncPlaylistItemId = addedDuringSync.id;
  await playlistEvent;
  const duringSync = await api(`/windows/${syncWindowId}`);
  assert.equal(duringSync.current_playback_state.mode, 'sync');
  assert.ok(duringSync.playlist.some((item) => item.id === syncPlaylistItemId));
  assert.deepEqual(
    (await api(`/windows/${windows[1].id}`)).playlist.map((item) => item.id),
    unaffectedBefore,
  );

  const endedEvents = sockets.map((socket) => waitForMessage(socket, (message) => message.type === 'SYNC_ENDED'));
  await api('/sync/cancel', { method: 'POST', body: '{}' });
  syncStarted = false;
  await Promise.all(endedEvents);

  const resumed = await Promise.all(windows.slice(0, 3).map((window) => api(`/windows/${window.id}`)));
  resumed.forEach((window) => assert.equal(window.current_playback_state.mode, 'normal'));
  assert.ok(resumed[0].playlist.some((item) => item.id === syncPlaylistItemId));

  await api(`/windows/${syncWindowId}/playlist/${syncPlaylistItemId}`, { method: 'DELETE' });
  syncPlaylistItemId = undefined;

  console.log(`Playback E2E passed: ${windows.length} windows, ${media.length} media items, sync-time playlist update and resume verified.`);
} finally {
  sockets.forEach((socket) => socket.close());
  if (syncStarted) await api('/sync/cancel', { method: 'POST', body: '{}' }).catch(() => {});
  if (syncPlaylistItemId && syncWindowId) {
    await api(`/windows/${syncWindowId}/playlist/${syncPlaylistItemId}`, { method: 'DELETE' }).catch(() => {});
  }
  if (temporaryWindowId) await api(`/windows/${temporaryWindowId}`, { method: 'DELETE' }).catch(() => {});
}
