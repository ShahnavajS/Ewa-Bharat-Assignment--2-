import React, { useState, useEffect } from 'react';
import { MediaItem, MediaType, CreateMediaInput, UpdateMediaInput, ApiError } from '../types';
import { createMedia, updateMedia } from '../api/media';

interface CreateEditMediaModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (item: MediaItem) => void;
  mediaToEdit?: MediaItem | null;
}

export const CreateEditMediaModal: React.FC<CreateEditMediaModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
  mediaToEdit,
}) => {
  const isEditing = Boolean(mediaToEdit);

  const [title, setTitle] = useState<string>('');
  const [type, setType] = useState<MediaType>('image');
  const [url, setUrl] = useState<string>('');
  const [duration, setDuration] = useState<number>(30);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (mediaToEdit) {
      setTitle(mediaToEdit.title);
      setType(mediaToEdit.type);
      setUrl(mediaToEdit.url);
      setDuration(mediaToEdit.default_duration_seconds);
    } else {
      setTitle('');
      setType('image');
      setUrl('');
      setDuration(30);
    }
    setError(null);
  }, [mediaToEdit, isOpen]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const cleanTitle = title.trim();
    if (!cleanTitle) {
      setError('Title is required');
      return;
    }

    if (duration <= 0 || isNaN(duration)) {
      setError('Duration must be a positive number');
      return;
    }

    let cleanUrl = url.trim();
    if (type === 'blank') {
      cleanUrl = cleanUrl || 'about:blank';
    } else if (!cleanUrl) {
      setError('URL is required for image and video assets');
      return;
    }

    setLoading(true);
    try {
      if (isEditing && mediaToEdit) {
        const payload: UpdateMediaInput = {
          title: cleanTitle,
          type,
          url: cleanUrl,
          default_duration_seconds: duration,
        };
        const updated = await updateMedia(mediaToEdit.id, payload);
        onSuccess(updated);
        onClose();
      } else {
        const payload: CreateMediaInput = {
          title: cleanTitle,
          type,
          url: cleanUrl,
          default_duration_seconds: duration,
        };
        const created = await createMedia(payload);
        onSuccess(created);
        onClose();
      }
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(`${err.code}: ${err.message}`);
      } else {
        setError('Failed to save media item');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-backdrop" onClick={onClose} role="dialog" aria-modal="true">
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3 className="modal-title">{isEditing ? 'Edit Media Asset' : 'Register New Media Asset'}</h3>
          <button className="modal-close-btn" onClick={onClose} aria-label="Close modal">Close</button>
        </div>

        {error && (
          <div className="modal-error-banner">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="modal-form">
          <div className="form-group">
            <label className="form-label" htmlFor="media-title">Title *</label>
            <input
              id="media-title"
              type="text"
              className="form-input"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="e.g. Welcome Announcement"
              disabled={loading}
              autoFocus
            />
          </div>

          <div className="form-row">
            <div className="form-group" style={{ flex: 1 }}>
              <label className="form-label" htmlFor="media-type">Media Type *</label>
              <select
                id="media-type"
                className="form-select"
                value={type}
                onChange={(e) => {
                  const newType = e.target.value as MediaType;
                  setType(newType);
                  if (newType === 'blank' && !url) {
                    setUrl('about:blank');
                  }
                }}
                disabled={loading}
              >
                <option value="image">Image (jpg, png, webp)</option>
                <option value="video">Video (mp4, webm)</option>
                <option value="blank">Blank Screen (Standby)</option>
              </select>
            </div>

            <div className="form-group" style={{ width: '140px' }}>
              <label className="form-label" htmlFor="media-duration">Default Duration (s) *</label>
              <input
                id="media-duration"
                type="number"
                min="1"
                className="form-input"
                value={duration}
                onChange={(e) => setDuration(parseInt(e.target.value, 10) || 0)}
                disabled={loading}
              />
            </div>
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="media-url">
              Asset URL {type === 'blank' ? '(Optional for blank screen)' : '*'}
            </label>
            <input
              id="media-url"
              type="text"
              className="form-input"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder={type === 'blank' ? 'about:blank' : 'https://example.com/asset.jpg'}
              disabled={loading}
            />
            <span className="form-hint">
              {type === 'blank'
                ? 'Blank screen renders an intentional standby display without external asset.'
                : 'Direct HTTPS media link or CDN asset URL.'}
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
              disabled={loading}
            >
              {loading ? 'Saving...' : isEditing ? 'Save Changes' : 'Register Asset'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
