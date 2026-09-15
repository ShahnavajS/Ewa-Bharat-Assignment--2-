import { useState, useEffect, useRef, useCallback } from 'react';
import { SequencerWebSocketClient } from '../websocket/client';
import { getWindow } from '../api/windows';
import {
  PlaybackState,
  PlaylistItem,
  Window,
  MediaItem,
  ConnectionStatus,
  ServerWebSocketMessage,
} from '../types';
import {
  isStateSnapshot,
  isPlaylistUpdated,
  isSyncStarted,
  isSyncEnded,
} from '../websocket/protocol';

export interface WindowPlaybackHook {
  loading: boolean;
  windowDetails: Window | null;
  playlist: PlaylistItem[];
  playbackState: PlaybackState | null;
  connectionStatus: ConnectionStatus;
  isSyncActive: boolean;
  syncMedia: MediaItem | null;
  localElapsedSeconds: number;
  localRemainingSeconds: number;
  error: string | null;
  refreshPlayback: () => Promise<void>;
}

/**
 * Hook managing real-time playback for a single display window.
 * Authoritative state is driven by the backend Go PlaybackEngine;
 * local smooth interpolation tracks item progress between synchronization ticks.
 */
export function useWindowPlayback(windowId: number): WindowPlaybackHook {
  const [loading, setLoading] = useState<boolean>(true);
  const [windowDetails, setWindowDetails] = useState<Window | null>(null);
  const [playlist, setPlaylist] = useState<PlaylistItem[]>([]);
  const [playbackState, setPlaybackState] = useState<PlaybackState | null>(null);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('connecting');
  const [isSyncActive, setIsSyncActive] = useState<boolean>(false);
  const [syncMedia, setSyncMedia] = useState<MediaItem | null>(null);
  const [localElapsedSeconds, setLocalElapsedSeconds] = useState<number>(0);
  const [localRemainingSeconds, setLocalRemainingSeconds] = useState<number>(0);
  const [error, setError] = useState<string | null>(null);

  // References for timer management and interpolation
  const clientRef = useRef<SequencerWebSocketClient | null>(null);
  const transitionTimerRef = useRef<number | null>(null);
  const localTickTimerRef = useRef<number | null>(null);
  const authoritativeServerMsRef = useRef<number>(Date.now());
  const authoritativeElapsedRef = useRef<number>(0);

  // Authoritative fetch from REST API
  const refreshPlayback = useCallback(async () => {
    try {
      const data = await getWindow(windowId);
      setWindowDetails(data);
      setPlaylist(data.playlist || []);
      const pb = data.current_playback_state;
      setPlaybackState(pb);

      if (pb.mode === 'sync') {
        setIsSyncActive(true);
        setSyncMedia(pb.media || null);
      } else {
        setIsSyncActive(false);
        setSyncMedia(null);
      }

      const elapsed = pb.elapsed_seconds ?? 0;
      const remaining = pb.remaining_seconds ?? 0;
      authoritativeElapsedRef.current = elapsed;
      const calculatedAt = Date.parse(pb.calculated_at);
      authoritativeServerMsRef.current = Number.isNaN(calculatedAt) ? Date.now() : calculatedAt;
      setLocalElapsedSeconds(elapsed);
      setLocalRemainingSeconds(remaining);
      setError(null);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to fetch window playback';
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, [windowId]);

  // Initial authoritative REST load for immediate instant render
  useEffect(() => {
    refreshPlayback();
  }, [refreshPlayback]);

  // Set up dedicated WebSocket client for this window
  useEffect(() => {
    const client = new SequencerWebSocketClient({
      autoConnect: true,
    });
    clientRef.current = client;

    const unsubStatus = client.onStatusChange((status) => {
      setConnectionStatus(status);
      if (status === 'connected') {
        client.subscribe(windowId);
      }
    });

    const unsubMessage = client.onMessage((msg: ServerWebSocketMessage) => {
      // 1. Handle STATE_SNAPSHOT
      if (isStateSnapshot(msg) && msg.window_id === windowId) {
        const payload = msg.playback || msg.data;
        if (payload) {
          setLoading(false);
          setWindowDetails(payload);
          setPlaylist(payload.playlist || []);
          const pb = payload.current_playback_state;
          if (pb) {
            setPlaybackState(pb);
            const isSync = pb.mode === 'sync';
            setIsSyncActive(isSync);
            setSyncMedia(isSync ? pb.media || null : null);

            const elapsed = pb.elapsed_seconds ?? 0;
            const remaining = pb.remaining_seconds ?? 0;
            authoritativeElapsedRef.current = elapsed;
            const calculatedAt = Date.parse(pb.calculated_at);
            authoritativeServerMsRef.current = Number.isNaN(calculatedAt) ? Date.now() : calculatedAt;
            setLocalElapsedSeconds(elapsed);
            setLocalRemainingSeconds(remaining);
          }
        }
      }

      // 2. Handle PLAYLIST_UPDATED (window-specific)
      else if (isPlaylistUpdated(msg) && msg.window_id === windowId) {
        if (Array.isArray(msg.playlist)) {
          setPlaylist(msg.playlist);
        }
        // Fetch updated playback state from backend
        refreshPlayback();
      }

      // 3. Handle global SYNC_STARTED
      else if (isSyncStarted(msg)) {
        setIsSyncActive(true);
        // If sync payload includes media, update immediately
        const syncData = (msg.sync || msg.data) as any;
        if (syncData?.media) {
          setSyncMedia(syncData.media);
        }
        // Refresh authoritative state to reflect sync mode
        refreshPlayback();
      }

      // 4. Handle global SYNC_ENDED
      else if (isSyncEnded(msg)) {
        setIsSyncActive(false);
        setSyncMedia(null);
        // Resume normal deterministic wall-clock playlist calculation immediately
        refreshPlayback();
      }
    });

    return () => {
      unsubStatus();
      unsubMessage();
      client.disconnect();
      clientRef.current = null;
    };
  }, [windowId, refreshPlayback]);

  // Smooth local timer interpolation & transition scheduler.
  // NOTE: deps MUST NOT include localRemainingSeconds — that value updates every second
  // via the interval below, and including it would recreate timers on every tick,
  // causing the transition setTimeout to reset perpetually and never fire.
  useEffect(() => {
    // Clear existing local ticker
    if (localTickTimerRef.current !== null) {
      clearInterval(localTickTimerRef.current);
      localTickTimerRef.current = null;
    }
    if (transitionTimerRef.current !== null) {
      clearTimeout(transitionTimerRef.current);
      transitionTimerRef.current = null;
    }

    if (!playbackState || !playbackState.active || playbackState.playlist_empty) {
      return;
    }

    const itemDuration =
      (playbackState.elapsed_seconds ?? 0) + (playbackState.remaining_seconds ?? 0);

    // Snapshot remaining seconds from authoritative state at the moment this effect runs.
    // Do NOT use localRemainingSeconds here — it updates every tick.
    const snapshotRemaining = playbackState.remaining_seconds ?? 0;

    // Schedule an authoritative refresh just after the item should end.
    const remainingMs = Math.max(snapshotRemaining * 1000, 250);
    transitionTimerRef.current = window.setTimeout(() => {
      transitionTimerRef.current = null;
      refreshPlayback();
    }, remainingMs + 250); // 250ms grace period for backend calculation

    // Local 1-second progress ticker for smooth UI progress bar.
    // When time expires inside the tick, clear the interval immediately to prevent
    // repeated refreshPlayback() calls every second while waiting for backend response.
    localTickTimerRef.current = window.setInterval(() => {
      const estimatedServerMs = clientRef.current?.getServerTime().getTime() ?? Date.now();
      const secondsSinceAuthoritative = Math.max(
        0,
        (estimatedServerMs - authoritativeServerMsRef.current) / 1000,
      );
      const estimatedElapsed = authoritativeElapsedRef.current + secondsSinceAuthoritative;

      if (itemDuration > 0) {
        const clampedElapsed = Math.min(estimatedElapsed, itemDuration);
        const clampedRemaining = Math.max(itemDuration - clampedElapsed, 0);
        setLocalElapsedSeconds(clampedElapsed);
        setLocalRemainingSeconds(clampedRemaining);

        if (clampedRemaining === 0) {
          // Stop the tick immediately — transition timer or the server will handle advancement.
          if (localTickTimerRef.current !== null) {
            clearInterval(localTickTimerRef.current);
            localTickTimerRef.current = null;
          }
        }
      }
    }, 250);

    return () => {
      if (localTickTimerRef.current !== null) {
        clearInterval(localTickTimerRef.current);
        localTickTimerRef.current = null;
      }
      if (transitionTimerRef.current !== null) {
        clearTimeout(transitionTimerRef.current);
        transitionTimerRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playbackState, refreshPlayback]);

  return {
    loading,
    windowDetails,
    playlist,
    playbackState,
    connectionStatus,
    isSyncActive,
    syncMedia,
    localElapsedSeconds,
    localRemainingSeconds,
    error,
    refreshPlayback,
  };
}
