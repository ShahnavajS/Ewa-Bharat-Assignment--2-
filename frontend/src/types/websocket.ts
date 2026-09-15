import { WindowPlaybackResponse } from './window';
import { PlaylistItem } from './playlist';

/**
 * Realtime connection status states.
 */
export type ConnectionStatus =
  | 'connecting'
  | 'connected'
  | 'disconnected'
  | 'reconnecting'
  | 'error';

/**
 * Inbound messages sent from client to server.
 */
export interface SubscribeMessage {
  type: 'SUBSCRIBE';
  window_id: number;
}

export type ClientWebSocketMessage = SubscribeMessage;

/**
 * Server-to-client message schemas.
 */
export interface StateSnapshotMessage {
  type: 'STATE_SNAPSHOT';
  server_time: string;
  timestamp: string;
  window_id: number;
  playback: WindowPlaybackResponse;
  data?: WindowPlaybackResponse;
}

export interface TimeSyncMessage {
  type: 'TIME_SYNC';
  server_time: string;
  timestamp: string;
  data?: {
    server_time: string;
  };
}

export interface PlaylistUpdatedMessage {
  type: 'PLAYLIST_UPDATED';
  server_time: string;
  timestamp: string;
  window_id: number;
  playlist: PlaylistItem[];
  data?: unknown;
}

export interface SyncStartedMessage {
  type: 'SYNC_STARTED';
  server_time: string;
  timestamp: string;
  sync?: unknown;
  data?: unknown;
}

export interface SyncEndedMessage {
  type: 'SYNC_ENDED';
  server_time: string;
  timestamp: string;
  sync?: unknown;
  data?: unknown;
}

export interface ErrorWebSocketMessage {
  type: 'ERROR';
  server_time: string;
  timestamp: string;
  code: string;
  message: string;
}

/**
 * Discriminated union of all server-to-client WebSocket messages.
 */
export type ServerWebSocketMessage =
  | StateSnapshotMessage
  | TimeSyncMessage
  | PlaylistUpdatedMessage
  | SyncStartedMessage
  | SyncEndedMessage
  | ErrorWebSocketMessage;
