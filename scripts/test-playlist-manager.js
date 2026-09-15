import { api, assert, delay, playlist, subscribe, waitForMessage } from './e2e-utils.js';

let addedItem;
let targetWindow;
let targetSocket;
let otherSocket;

try {
  const windows = await api('/windows');
  const media = await api('/media');
  assert.ok(windows.length >= 2, 'expected at least two windows');
  assert.ok(media.length > 0, 'expected seeded media');

  targetWindow = windows[0];
  const otherWindow = windows[1];
  const original = await playlist(targetWindow.id);
  const otherOriginal = await playlist(otherWindow.id);

  targetSocket = await subscribe(targetWindow.id);
  otherSocket = await subscribe(otherWindow.id);
  let unrelatedUpdates = 0;
  otherSocket.addEventListener('message', (event) => {
    const message = JSON.parse(String(event.data));
    if (message.type === 'PLAYLIST_UPDATED') unrelatedUpdates += 1;
  });

  const addEvent = waitForMessage(
    targetSocket,
    (message) => message.type === 'PLAYLIST_UPDATED' && message.window_id === targetWindow.id,
  );
  addedItem = await api(`/windows/${targetWindow.id}/playlist`, {
    method: 'POST',
    body: JSON.stringify({ media_id: media[0].id, duration_seconds: 7 }),
  });
  await addEvent;

  const afterAdd = await playlist(targetWindow.id);
  assert.equal(afterAdd.length, original.length + 1);
  assert.equal(afterAdd.at(-1).id, addedItem.id);

  const reorderedIds = [addedItem.id, ...original.map((item) => item.id)];
  const reorderEvent = waitForMessage(targetSocket, (message) => message.type === 'PLAYLIST_UPDATED');
  const reorderedResponse = await api(`/windows/${targetWindow.id}/playlist/reorder`, {
    method: 'PATCH',
    body: JSON.stringify({ item_ids: reorderedIds }),
  });
  await reorderEvent;
  const reordered = Array.isArray(reorderedResponse) ? reorderedResponse : reorderedResponse.items;
  assert.deepEqual(reordered.map((item) => item.id), reorderedIds);

  const deleteEvent = waitForMessage(targetSocket, (message) => message.type === 'PLAYLIST_UPDATED');
  await api(`/windows/${targetWindow.id}/playlist/${addedItem.id}`, { method: 'DELETE' });
  await deleteEvent;
  addedItem = undefined;

  const restored = await playlist(targetWindow.id);
  const otherAfter = await playlist(otherWindow.id);
  assert.deepEqual(restored.map((item) => item.id), original.map((item) => item.id));
  assert.deepEqual(otherAfter.map((item) => item.id), otherOriginal.map((item) => item.id));
  await delay(250);
  assert.equal(unrelatedUpdates, 0, 'unaffected window received a playlist update');

  console.log('Playlist E2E passed: add, persistence, realtime update, reorder, remove, isolation, and cleanup verified.');
} finally {
  targetSocket?.close();
  otherSocket?.close();
  if (addedItem && targetWindow) {
    await api(`/windows/${targetWindow.id}/playlist/${addedItem.id}`, { method: 'DELETE' }).catch(() => {});
  }
}
