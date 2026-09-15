import { PlaylistItem } from './playlist';
import { PlaybackState } from './playback';

/**
 * Window represents an independent display screen configuration.
 */
export interface Window {
  id: number;
  name: string;
  description: string;
  cycle_duration_seconds: number;
  epoch_start_time: string;
  created_at: string;
  updated_at: string;
  playlist_items?: PlaylistItem[];
}

/**
 * WindowPlaybackResponse bundles window details, ordered playlist, and calculated playback state.
 */
export interface WindowPlaybackResponse extends Window {
  playlist: PlaylistItem[];
  current_playback_state: PlaybackState;
}

/**
 * Input payload for creating a display window.
 */
export interface CreateWindowInput {
  name: string;
  description?: string;
  cycle_duration_seconds?: number;
}

/**
 * Input payload for partially updating a display window.
 */
export interface UpdateWindowInput {
  name?: string;
  description?: string;
  cycle_duration_seconds?: number;
}
