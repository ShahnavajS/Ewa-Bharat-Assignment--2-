import React, { useState } from 'react';
import { MediaItem, ApiError } from '../types';
import { deleteMedia } from '../api/media';
import { CreateEditMediaModal } from './CreateEditMediaModal';
import { ConfirmDeleteDialog } from './ConfirmDeleteDialog';

interface MediaLibraryManagerProps {
  mediaList: MediaItem[];
  loading: boolean;
  onRefresh: () => void;
}

export const MediaLibraryManager: React.FC<MediaLibraryManagerProps> = ({
  mediaList,
  loading,
  onRefresh,
}) => {
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false);
  const [editingMedia, setEditingMedia] = useState<MediaItem | null>(null);

  // Deletion state
  const [deletingMedia, setDeletingMedia] = useState<MediaItem | null>(null);
  const [deleteLoading, setDeleteLoading] = useState<boolean>(false);
  const [errorAlert, setErrorAlert] = useState<string | null>(null);

  const handleOpenCreate = () => {
    setEditingMedia(null);
    setIsModalOpen(true);
  };

  const handleOpenEdit = (item: MediaItem) => {
    setEditingMedia(item);
    setIsModalOpen(true);
  };

  const handleConfirmDelete = async () => {
    if (!deletingMedia) return;
    setDeleteLoading(true);
    setErrorAlert(null);
    try {
      await deleteMedia(deletingMedia.id);
      setDeletingMedia(null);
      onRefresh();
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 409 || err.code === 'MEDIA_IN_USE') {
          setErrorAlert(`Cannot delete "${deletingMedia.title}" because it is currently assigned to one or more playlists.`);
        } else {
          setErrorAlert(`${err.code}: ${err.message}`);
        }
      } else {
        setErrorAlert('Failed to delete media asset');
      }
      setDeletingMedia(null);
    } finally {
      setDeleteLoading(false);
    }
  };

  return (
    <div className="card media-library-card">
      <div className="card-title">
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span>Media Assets Library</span>
          <span className="count-badge">{mediaList.length}</span>
        </div>
        <button
          className="btn btn-primary btn-sm"
          onClick={handleOpenCreate}
        >
          + Register Media
        </button>
      </div>

      {errorAlert && (
        <div className="error-banner" style={{ margin: '10px 0 16px' }}>
          <div>{errorAlert}</div>
          <button className="btn btn-secondary btn-sm" onClick={() => setErrorAlert(null)}>
            Dismiss
          </button>
        </div>
      )}

      {loading && mediaList.length === 0 ? (
        <div className="manager-empty-state">
          <div className="spinner" />
          <p>Loading media library...</p>
        </div>
      ) : mediaList.length === 0 ? (
        <div className="manager-empty-state">
          <p>No media assets registered yet. Click "+ Register Media" to add your first asset.</p>
        </div>
      ) : (
        <div className="media-grid">
          {mediaList.map((item) => (
            <div key={item.id} className="media-card-item">
              {/* Media Thumbnail Preview */}
              <div className="media-thumbnail-box">
                {item.type === 'blank' ? (
                  <div className="thumb-blank">
                    <span className="blank-dot" />
                    <span>STANDBY</span>
                  </div>
                ) : item.type === 'video' ? (
                  <div className="thumb-video-box">
                    <video src={item.url} muted preload="metadata" className="thumb-video-elem" />
                    <span className="thumb-play-overlay">VIDEO</span>
                  </div>
                ) : (
                  <img
                    src={item.url}
                    alt={item.title}
                    className="thumb-image-elem"
                    loading="lazy"
                    onError={(e) => {
                      (e.target as HTMLImageElement).src = 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="60" height="40"><rect width="100%" height="100%" fill="%23222"/><text x="50%" y="50%" fill="%23888" font-size="10" text-anchor="middle" dy=".3em">Broken</text></svg>';
                    }}
                  />
                )}
                <span className={`type-pill ${item.type} thumb-badge`}>{item.type}</span>
              </div>

              {/* Media Info */}
              <div className="media-info-block">
                <h4 className="media-card-title" title={item.title}>
                  {item.title}
                </h4>
                <div className="media-card-meta">
                  <span className="font-mono">{item.default_duration_seconds}s</span>
                  <span className="meta-sep">•</span>
                  <span className="media-card-url" title={item.url}>
                    {item.type === 'blank' ? 'intentional blank' : item.url}
                  </span>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="media-card-actions">
                <button
                  className="btn btn-secondary btn-sm"
                  onClick={() => handleOpenEdit(item)}
                  title="Edit media"
                  aria-label={`Edit ${item.title}`}
                >
                  Edit
                </button>
                <button
                  className="btn btn-danger btn-sm"
                  onClick={() => setDeletingMedia(item)}
                  title="Delete media"
                  aria-label={`Delete ${item.title}`}
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Create / Edit Modal */}
      <CreateEditMediaModal
        isOpen={isModalOpen}
        mediaToEdit={editingMedia}
        onClose={() => setIsModalOpen(false)}
        onSuccess={() => {
          onRefresh();
        }}
      />

      {/* Delete Confirmation Dialog */}
      <ConfirmDeleteDialog
        isOpen={Boolean(deletingMedia)}
        title="Delete Media Asset"
        message={`Are you sure you want to permanently delete "${deletingMedia?.title}"? If this asset is assigned to any window playlist, deletion will be rejected.`}
        confirmLabel="Delete Asset"
        loading={deleteLoading}
        onConfirm={handleConfirmDelete}
        onCancel={() => setDeletingMedia(null)}
      />
    </div>
  );
};
