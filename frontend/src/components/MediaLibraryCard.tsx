import React from 'react';
import { MediaItem } from '../types';

interface MediaLibraryCardProps {
  media: MediaItem[];
  loading: boolean;
  onRefresh: () => void;
}

export const MediaLibraryCard: React.FC<MediaLibraryCardProps> = ({
  media,
  loading,
  onRefresh,
}) => {
  return (
    <div className="card" style={{ gridColumn: 'span 6' }}>
      <div className="card-title">
        <span>Media Assets Library ({media.length})</span>
        <button
          className="btn btn-secondary"
          onClick={onRefresh}
          disabled={loading}
          style={{ fontSize: '0.75rem', padding: '3px 8px' }}
        >
          {loading ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {loading && media.length === 0 ? (
        <div style={{ color: 'var(--text-muted)', fontSize: '0.9rem', padding: '16px 0' }}>
          Loading media library...
        </div>
      ) : media.length === 0 ? (
        <div style={{ color: 'var(--text-muted)', fontSize: '0.9rem', padding: '16px 0' }}>
          No media assets found in library.
        </div>
      ) : (
        <div className="media-list">
          {media.map((item) => (
            <div key={item.id} className="media-item-row">
              <div className="media-title">
                <span className={`type-pill ${item.type}`}>
                  {item.type}
                </span>
                <span>{item.title}</span>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <span style={{ fontSize: '0.78rem', color: 'var(--text-muted)' }}>
                  {item.default_duration_seconds}s
                </span>
                <span className="code-snippet" title={item.url}>
                  #{item.id}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
