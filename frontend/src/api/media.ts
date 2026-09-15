import { apiClient } from './client';
import { MediaItem, CreateMediaInput, UpdateMediaInput } from '../types';

/**
 * Fetch all media items in the global media library.
 */
export async function getMedia(): Promise<MediaItem[]> {
  return apiClient.get<MediaItem[]>('/media');
}

/**
 * Fetch a single media item by ID.
 */
export async function getMediaById(id: number): Promise<MediaItem> {
  return apiClient.get<MediaItem>(`/media/${id}`);
}

/**
 * Register a new media asset.
 */
export async function createMedia(payload: CreateMediaInput): Promise<MediaItem> {
  return apiClient.post<MediaItem>('/media', payload);
}

/**
 * Partially update an existing media asset.
 */
export async function updateMedia(id: number, payload: UpdateMediaInput): Promise<MediaItem> {
  return apiClient.patch<MediaItem>(`/media/${id}`, payload);
}

/**
 * Delete a media asset by ID.
 */
export async function deleteMedia(id: number): Promise<{ message: string }> {
  return apiClient.delete<{ message: string }>(`/media/${id}`);
}
