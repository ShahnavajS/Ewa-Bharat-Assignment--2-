import React, { useState, useEffect, useCallback } from 'react';
import { Window, PlaylistItem, MediaItem, ApiError, ServerWebSocketMessage } from '../types';
import { getPlaylist, deletePlaylistItem, reorderPlaylist } from '../api/playlists';
import { SequencerWebSocketClient } from '../websocket/client';
import { isPlaylistUpdated } from '../websocket/protocol';
import { AddToPlaylistModal } from './AddToPlaylistModal';
import { ConfirmDeleteDialog } from './ConfirmDeleteDialog';

interface PlaylistEditorProps {
  windows: Window[];
  selectedWindowId: number;
  onSelectWindow: (id: number) => void;
  mediaList: MediaItem[];
}

export const PlaylistEditor: React.FC<PlaylistEditorProps> = ({
  windows,
  selectedWindowId,
  onSelectWindow,
  mediaList,
}) => {
  const [playlist, setPlaylist] = useState<PlaylistItem[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [isAddModalOpen, setIsAddModalOpen] = useState<boolean>(false);

  // Mutation states
  const [reordering, setReordering] = useState<boolean>(false);
  const [deletingItem, setDeletingItem] = useState<PlaylistItem | null>(null);
  const [deleteLoading, setDeleteLoading] = useState<boolean>(false);

  const selectedWindow = windows.find((w) => w.id === selectedWindowId);

  // Fetch playlist from authoritative REST API
  const fetchPlaylist = useCallback(async () => {
    if (!selectedWindowId) {
      setPlaylist([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const items = await getPlaylist(selectedWindowId);
      setPlaylist(items || []);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(`${err.code}: ${err.message}`);
      } else {
        setError('Failed to load playlist');
      }
    } finally {
      setLoading(false);
    }
  }, [selectedWindowId]);

  useEffect(() => {
    fetchPlaylist();
  }, [fetchPlaylist]);

  // Real-time WebSocket listener: auto-refresh when PLAYLIST_UPDATED arrives for this window
  useEffect(() => {
    if (!selectedWindowId) return;
    const client = new SequencerWebSocketClient({ autoConnect: true });
    const unsubStatus = client.onStatusChange((status) => {
      if (status === 'connected') client.subscribe(selectedWindowId);
    });
    const unsub = client.onMessage((msg: ServerWebSocketMessage) => {
      if (isPlaylistUpdated(msg) && msg.window_id === selectedWindowId) {
        if (Array.isArray(msg.playlist)) {
          setPlaylist(msg.playlist);
        } else {
          fetchPlaylist();
        }
      }
    });

    return () => {
      unsubStatus();
      unsub();
      client.disconnect();
    };
  }, [selectedWindowId, fetchPlaylist]);

  // Reorder Handler: Move item up or down
  const handleMove = async (index: number, direction: 'up' | 'down') => {
    if (reordering) return;
    const targetIndex = direction === 'up' ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= playlist.length) return;

    // Create reordered item IDs list
    const updated = [...playlist];
    const temp = updated[index];
    updated[index] = updated[targetIndex];
    updated[targetIndex] = temp;

    const itemIds = updated.map((item) => item.id);

    setReordering(true);
    setError(null);
    try {
      const result = await reorderPlaylist(selectedWindowId, itemIds);
      setPlaylist(result || updated);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(`${err.code}: ${err.message}`);
      } else {
        setError('Failed to reorder playlist');
      }
      // Re-fetch authoritative state on error
      fetchPlaylist();
    } finally {
      setReordering(false);
    }
  };

  // Remove Item Handler
  const handleConfirmRemove = async () => {
    if (!deletingItem) return;
    setDeleteLoading(true);
    setError(null);
    try {
      await deletePlaylistItem(selectedWindowId, deletingItem.id);
      setDeletingItem(null);
      fetchPlaylist();
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(`${err.code}: ${err.message}`);
      } else {
        setError('Failed to remove playlist item');
      }
      setDeletingItem(null);
    } finally {
      setDeleteLoading(false);
    }
  };

  const totalDuration = playlist.reduce((sum, item) => sum + item.duration_seconds, 0);

  return (
    <div className="card playlist-editor-card">
      {/* Window Selector & Header */}
      <div className="playlist-header-block">
        <div className="window-select-group">
          <label className="form-label" htmlFor="window-picker" style={{ marginBottom: 0 }}>
            Display Window:
          </label>
          <select
            id="window-picker"
            className="form-select window-select"
            value={selectedWindowId}
            onChange={(e) => onSelectWindow(parseInt(e.target.value, 10))}
            disabled={windows.length === 0}
          >
            {windows.length === 0 && <option value={0}>No display windows</option>}
            {windows.map((win) => (
              <option key={win.id} value={win.id}>
                Window #{win.id}: {win.name}
              </option>
            ))}
          </select>
        </div>

        <button
          className="btn btn-primary btn-sm"
          onClick={() => setIsAddModalOpen(true)}
          disabled={!selectedWindowId || mediaList.length === 0}
        >
          + Add Media to Sequence
        </button>
      </div>

      {/* Window Stats Pill */}
      {selectedWindow && (
        <div className="playlist-stats-bar">
          <div className="stat-pill">
            <span className="stat-pill-label">Cycle Duration:</span>
            <span className="stat-pill-val font-mono">
              {(selectedWindow.cycle_duration_seconds / 3600).toFixed(1).replace('.0', '')}h
            </span>
          </div>
          <div className="stat-pill">
            <span className="stat-pill-label">Playlist Total:</span>
            <span className="stat-pill-val font-mono" style={{ color: '#34d399' }}>{totalDuration}s</span>
          </div>
          <div className="stat-pill">
            <span className="stat-pill-label">Items:</span>
            <span className="stat-pill-val font-mono">{playlist.length}</span>
          </div>
        </div>
      )}

      {error && (
        <div className="error-banner" style={{ margin: '12px 0' }}>
          <div>{error}</div>
          <button className="btn btn-secondary btn-sm" onClick={fetchPlaylist}>
            Retry
          </button>
        </div>
      )}

      {/* Playlist Items List */}
      {!selectedWindowId ? (
        <div className="manager-empty-state">
          <h4>No display windows</h4>
          <p>Create a display window before building a playlist.</p>
        </div>
      ) : loading && playlist.length === 0 ? (
        <div className="manager-empty-state">
          <div className="spinner" />
          <p>Loading playlist for window #{selectedWindowId}...</p>
        </div>
      ) : playlist.length === 0 ? (
        <div className="manager-empty-state">
          <h4>Playlist Empty</h4>
          <p>This display window has no playlist configured. Click "+ Add Media to Sequence" above.</p>
        </div>
      ) : (
        <div className="playlist-table-container">
          <table className="playlist-table">
            <thead>
              <tr>
                <th style={{ width: '50px' }}>#</th>
                <th>Media Item</th>
                <th style={{ width: '90px' }}>Type</th>
                <th style={{ width: '130px' }}>Duration</th>
                <th style={{ width: '140px', textAlign: 'center' }}>Reorder</th>
                <th style={{ width: '80px', textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {playlist.map((item, index) => {
                const mediaTitle = item.media?.title || `Media Asset #${item.media_item_id}`;
                const mediaType = item.media?.type || 'unknown';
                const isFirst = index === 0;
                const isLast = index === playlist.length - 1;

                return (
                  <tr key={item.id} className="playlist-row">
                    {/* Position */}
                    <td className="font-mono text-muted font-bold">
                      #{item.position}
                    </td>

                    {/* Media Title & URL */}
                    <td>
                      <div className="table-media-info">
                        <span className="table-media-title" title={mediaTitle}>
                          {mediaTitle}
                        </span>
                        {item.media?.url && (
                          <span className="table-media-url" title={item.media.url}>
                            {mediaType === 'blank' ? 'intentional blank' : item.media.url}
                          </span>
                        )}
                      </div>
                    </td>

                    {/* Media Type */}
                    <td>
                      <span className={`type-pill ${mediaType}`}>{mediaType}</span>
                    </td>

                    {/* Duration (Read-only since backend does not expose item duration PATCH) */}
                    <td>
                      <div
                        className="duration-display-cell"
                        title="Configured duration for this item. Set upon item addition."
                      >
                        <span className="font-mono">{item.duration_seconds}s</span>
                        <span className="duration-lock-pill" title="Duration is fixed on append">Fixed</span>
                      </div>
                    </td>

                    {/* Reorder Buttons */}
                    <td style={{ textAlign: 'center' }}>
                      <div className="reorder-btn-group">
                        <button
                          className="reorder-btn"
                          disabled={isFirst || reordering}
                          onClick={() => handleMove(index, 'up')}
                          title={isFirst ? 'Already at top' : 'Move up'}
                          aria-label="Move playlist item up"
                        >
                          ▲
                        </button>
                        <button
                          className="reorder-btn"
                          disabled={isLast || reordering}
                          onClick={() => handleMove(index, 'down')}
                          title={isLast ? 'Already at bottom' : 'Move down'}
                          aria-label="Move playlist item down"
                        >
                          ▼
                        </button>
                      </div>
                    </td>

                    {/* Remove Action */}
                    <td style={{ textAlign: 'right' }}>
                      <button
                        className="btn btn-danger btn-sm"
                        onClick={() => setDeletingItem(item)}
                        title="Remove playlist item"
                        aria-label={`Remove ${mediaTitle} from sequence`}
                      >
                        Remove
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Add Media Modal */}
      <AddToPlaylistModal
        isOpen={isAddModalOpen}
        onClose={() => setIsAddModalOpen(false)}
        windowId={selectedWindowId}
        windowName={selectedWindow?.name || `Window #${selectedWindowId}`}
        mediaList={mediaList}
        onSuccess={() => {
          fetchPlaylist();
        }}
      />

      {/* Confirm Delete Playlist Item */}
      <ConfirmDeleteDialog
        isOpen={Boolean(deletingItem)}
        title="Remove Item from Playlist"
        message={`Remove "${deletingItem?.media?.title || `Item #${deletingItem?.position}`}" from ${selectedWindow?.name}? Other items will automatically compact positions.`}
        confirmLabel="Remove Item"
        loading={deleteLoading}
        onConfirm={handleConfirmRemove}
        onCancel={() => setDeletingItem(null)}
      />
    </div>
  );
};
