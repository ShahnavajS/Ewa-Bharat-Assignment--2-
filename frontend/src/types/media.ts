/**
 * Supported media format types in EVA Bharat Media Sequencer.
 */
export type MediaType = 'image' | 'video' | 'blank';

/**
 * MediaItem represents a registered asset in the media library.
 */
export interface MediaItem {
  id: number;
  title: string;
  type: MediaType;
  url: string;
  default_duration_seconds: number;
  created_at: string;
  updated_at: string;
}

/**
 * Payload for registering a new media asset.
 */
export interface CreateMediaInput {
  title: string;
  type: MediaType;
  url: string;
  default_duration_seconds: number;
}

/**
 * Payload for updating an existing media asset.
 */
export interface UpdateMediaInput {
  title?: string;
  type?: MediaType;
  url?: string;
  default_duration_seconds?: number;
}
