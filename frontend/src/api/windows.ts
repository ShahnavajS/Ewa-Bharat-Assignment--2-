import { apiClient } from './client';
import { Window, WindowPlaybackResponse, CreateWindowInput, UpdateWindowInput } from '../types';

/**
 * Fetch all display windows.
 */
export async function getWindows(): Promise<Window[]> {
  return apiClient.get<Window[]>('/windows');
}

/**
 * Fetch a single display window with its ordered playlist and calculated playback state.
 */
export async function getWindow(id: number): Promise<WindowPlaybackResponse> {
  return apiClient.get<WindowPlaybackResponse>(`/windows/${id}`);
}

/**
 * Create a new display window.
 */
export async function createWindow(payload: CreateWindowInput): Promise<Window> {
  return apiClient.post<Window>('/windows', payload);
}

/**
 * Partially update a display window.
 */
export async function updateWindow(id: number, payload: UpdateWindowInput): Promise<Window> {
  return apiClient.patch<Window>(`/windows/${id}`, payload);
}

/**
 * Delete a display window (cascades to delete its playlist).
 */
export async function deleteWindow(id: number): Promise<{ message: string }> {
  return apiClient.delete<{ message: string }>(`/windows/${id}`);
}
