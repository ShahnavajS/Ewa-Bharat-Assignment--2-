import { apiClient } from './client';
import {
  PlaylistItem,
  PlaylistResponse,
  AddPlaylistItemInput,
  ReorderPlaylistInput,
} from '../types';

/**
 * Fetch the ordered playlist for a specific display window.
 * Unwraps the backend PlaylistResponse envelope { window_id, items: [] }.
 */
export async function getPlaylist(windowId: number): Promise<PlaylistItem[]> {
  const res = await apiClient.get<PlaylistItem[] | PlaylistResponse>(
    `/windows/${windowId}/playlist`
  );
  if (Array.isArray(res)) return res;
  if (res && Array.isArray((res as PlaylistResponse).items)) {
    return (res as PlaylistResponse).items;
  }
  return [];
}

/**
 * Append a media item to a window's playlist.
 */
export async function addPlaylistItem(
  windowId: number,
  payload: AddPlaylistItemInput
): Promise<PlaylistItem> {
  return apiClient.post<PlaylistItem>(`/windows/${windowId}/playlist`, payload);
}

/**
 * Remove an item from a window's playlist (automatically recompacts positions).
 */
export async function deletePlaylistItem(
  windowId: number,
  itemId: number
): Promise<{ message: string }> {
  return apiClient.delete<{ message: string }>(`/windows/${windowId}/playlist/${itemId}`);
}

/**
 * Reorder a window's playlist items by providing the full sequence of item IDs.
 * Unwraps the updated items from PlaylistResponse.
 */
export async function reorderPlaylist(
  windowId: number,
  itemIds: number[]
): Promise<PlaylistItem[]> {
  const payload: ReorderPlaylistInput = { item_ids: itemIds };
  const res = await apiClient.patch<PlaylistItem[] | PlaylistResponse>(
    `/windows/${windowId}/playlist/reorder`,
    payload
  );
  if (Array.isArray(res)) return res;
  if (res && Array.isArray((res as PlaylistResponse).items)) {
    return (res as PlaylistResponse).items;
  }
  return [];
}
