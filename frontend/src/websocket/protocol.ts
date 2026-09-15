import {
  ServerWebSocketMessage,
  StateSnapshotMessage,
  TimeSyncMessage,
  PlaylistUpdatedMessage,
  SyncStartedMessage,
  SyncEndedMessage,
  ErrorWebSocketMessage,
  SubscribeMessage,
} from '../types/websocket';

export const WS_MSG_TYPES = {
  // Client -> Server
  SUBSCRIBE: 'SUBSCRIBE',

  // Server -> Client
  STATE_SNAPSHOT: 'STATE_SNAPSHOT',
  TIME_SYNC: 'TIME_SYNC',
  PLAYLIST_UPDATED: 'PLAYLIST_UPDATED',
  SYNC_STARTED: 'SYNC_STARTED',
  SYNC_ENDED: 'SYNC_ENDED',
  ERROR: 'ERROR',
} as const;

export function createSubscribeMessage(windowId: number): SubscribeMessage {
  return {
    type: 'SUBSCRIBE',
    window_id: windowId,
  };
}

/**
 * Type guard for STATE_SNAPSHOT message.
 */
export function isStateSnapshot(msg: ServerWebSocketMessage): msg is StateSnapshotMessage {
  return msg.type === WS_MSG_TYPES.STATE_SNAPSHOT;
}

/**
 * Type guard for TIME_SYNC message.
 */
export function isTimeSync(msg: ServerWebSocketMessage): msg is TimeSyncMessage {
  return msg.type === WS_MSG_TYPES.TIME_SYNC;
}

/**
 * Type guard for PLAYLIST_UPDATED message.
 */
export function isPlaylistUpdated(msg: ServerWebSocketMessage): msg is PlaylistUpdatedMessage {
  return msg.type === WS_MSG_TYPES.PLAYLIST_UPDATED;
}

/**
 * Type guard for SYNC_STARTED message.
 */
export function isSyncStarted(msg: ServerWebSocketMessage): msg is SyncStartedMessage {
  return msg.type === WS_MSG_TYPES.SYNC_STARTED;
}

/**
 * Type guard for SYNC_ENDED message.
 */
export function isSyncEnded(msg: ServerWebSocketMessage): msg is SyncEndedMessage {
  return msg.type === WS_MSG_TYPES.SYNC_ENDED;
}

/**
 * Type guard for ERROR message.
 */
export function isErrorMessage(msg: ServerWebSocketMessage): msg is ErrorWebSocketMessage {
  return msg.type === WS_MSG_TYPES.ERROR;
}
