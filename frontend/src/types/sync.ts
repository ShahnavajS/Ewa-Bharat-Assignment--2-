import { MediaItem } from './media';

/**
 * Status of a synchronization event.
 */
export type SyncStatus = 'active' | 'completed' | 'cancelled';

/**
 * SyncEvent represents an override event record in the database.
 */
export interface SyncEvent {
  id: number;
  media_item_id: number;
  duration_seconds: number;
  started_at: string;
  ends_at: string;
  triggered_by: string;
  status: SyncStatus;
  created_at: string;
  media?: MediaItem;
}

/**
 * ActiveSyncState represents the in-memory active sync status.
 */
export interface ActiveSyncState {
  active: boolean;
  media?: MediaItem;
  started_at?: string;
  ends_at?: string;
  duration_seconds?: number;
  remaining_seconds?: number;
  server_time?: string;
  sync_id?: number;
}

/**
 * Input for initiating global sync playback.
 */
export interface StartSyncInput {
  media_id: number;
  duration_seconds: number;
  triggered_by?: string;
}
