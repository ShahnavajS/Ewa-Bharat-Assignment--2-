import { MediaItem } from './media';

/**
 * PlaylistItem represents an entry in a specific window's sequence.
 */
export interface PlaylistItem {
  id: number;
  window_id: number;
  media_item_id: number;
  position: number;
  duration_seconds: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  media?: MediaItem;
}

/**
 * Response returned by playlist endpoints: GET and PATCH /reorder.
 */
export interface PlaylistResponse {
  window_id: number;
  items: PlaylistItem[];
}

/**
 * Payload to append a media item to a window's playlist.
 */
export interface AddPlaylistItemInput {
  media_id: number;
  duration_seconds?: number;
}

/**
 * Payload to reorder a window's playlist items.
 */
export interface ReorderPlaylistInput {
  item_ids: number[];
}
