import { apiClient } from './client';
import { SyncEvent, ActiveSyncState, StartSyncInput } from '../types';

/**
 * Trigger global synchronized playback override across all display windows.
 */
export async function startSync(payload: StartSyncInput): Promise<SyncEvent> {
  return apiClient.post<SyncEvent>('/sync', payload);
}

/**
 * Retrieve the current active global sync state.
 */
export async function getSyncState(): Promise<ActiveSyncState> {
  return apiClient.get<ActiveSyncState>('/sync/state');
}

/**
 * Prematurely cancel an active global synchronized playback override.
 */
export async function cancelSync(): Promise<{ message: string; data?: unknown }> {
  return apiClient.post<{ message: string; data?: unknown }>('/sync/cancel');
}
