import React, { useState, useEffect } from 'react';
import { MediaItem, PlaylistItem, ApiError } from '../types';
import { addPlaylistItem } from '../api/playlists';

interface AddToPlaylistModalProps {
  isOpen: boolean;
  onClose: () => void;
  windowId: number;
  windowName: string;
  mediaList: MediaItem[];
  onSuccess: (item: PlaylistItem) => void;
}

export const AddToPlaylistModal: React.FC<AddToPlaylistModalProps> = ({
  isOpen,
  onClose,
  windowId,
  windowName,
  mediaList,
  onSuccess,
}) => {
  const [selectedMediaId, setSelectedMediaId] = useState<number>(0);
  const [duration, setDuration] = useState<number>(30);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (mediaList.length > 0) {
      setSelectedMediaId(mediaList[0].id);
      setDuration(mediaList[0].default_duration_seconds);
    }
    setError(null);
  }, [mediaList, isOpen]);

  if (!isOpen) return null;

  const handleMediaChange = (id: number) => {
    setSelectedMediaId(id);
    const chosen = mediaList.find((m) => m.id === id);
    if (chosen) {
      setDuration(chosen.default_duration_seconds);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedMediaId) {
      setError('Please select a media item');
      return;
    }
    if (duration <= 0 || isNaN(duration)) {
      setError('Duration must be greater than 0 seconds');
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const added = await addPlaylistItem(windowId, {
        media_id: selectedMediaId,
        duration_seconds: duration,
      });
      onSuccess(added);
      onClose();
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(`${err.code}: ${err.message}`);
      } else {
        setError('Failed to append item to playlist');
      }
    } finally {
      setLoading(false);
    }
  };

  const chosenMedia = mediaList.find((m) => m.id === selectedMediaId);

  return (
    <div className="modal-backdrop" onClick={onClose} role="dialog" aria-modal="true">
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3 className="modal-title">Add Media to "{windowName}"</h3>
          <button className="modal-close-btn" onClick={onClose} aria-label="Close modal">Close</button>
        </div>

        {error && (
          <div className="modal-error-banner">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="modal-form">
          <div className="form-group">
            <label className="form-label" htmlFor="select-media">Select Media Asset *</label>
            <select
              id="select-media"
              className="form-select"
              value={selectedMediaId}
              onChange={(e) => handleMediaChange(parseInt(e.target.value, 10))}
              disabled={loading}
            >
              {mediaList.map((item) => (
                <option key={item.id} value={item.id}>
                  [{item.type.toUpperCase()}] {item.title} ({item.default_duration_seconds}s)
                </option>
              ))}
            </select>
          </div>

          {chosenMedia && (
            <div className="media-preview-box">
              <span className={`type-pill ${chosenMedia.type}`}>{chosenMedia.type}</span>
              <span className="media-preview-title">{chosenMedia.title}</span>
              <span className="form-hint" style={{ marginTop: 0 }}>
                Default: {chosenMedia.default_duration_seconds}s
              </span>
            </div>
          )}

          <div className="form-group">
            <label className="form-label" htmlFor="item-duration">Playlist Duration (seconds) *</label>
            <input
              id="item-duration"
              type="number"
              min="1"
              className="form-input"
              value={duration}
              onChange={(e) => setDuration(parseInt(e.target.value, 10) || 0)}
              disabled={loading}
            />
            <span className="form-hint">
              You can override the asset's default duration specifically for this window sequence.
            </span>
          </div>

          <div className="modal-actions">
            <button
              type="button"
              className="btn btn-secondary"
              onClick={onClose}
              disabled={loading}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={loading || mediaList.length === 0}
            >
              {loading ? 'Adding...' : 'Add to Playlist'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
