import { MediaItem } from './media';

/**
 * Playback operating mode.
 */
export type PlaybackMode = 'normal' | 'sync' | 'inactive';

/**
 * PlaybackState represents the result of deterministic calculation by the backend engine.
 */
export interface PlaybackState {
  mode: PlaybackMode;
  sync_event_id?: number;
  started: boolean;
  active: boolean;
  playlist_empty: boolean;
  window_id: number;
  playlist_item_id?: number;
  position?: number;
  media?: MediaItem;
  elapsed_seconds?: number;
  remaining_seconds?: number;
  cycle_position_seconds?: number;
  total_playlist_duration?: number;
  calculated_at: string;
}
