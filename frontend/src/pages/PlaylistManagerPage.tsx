import React, { useState, useEffect } from 'react';
import { useWindows } from '../hooks/useWindows';
import { useMedia } from '../hooks/useMedia';
import { PlaylistEditor } from '../components/PlaylistEditor';
import { MediaLibraryManager } from '../components/MediaLibraryManager';

export const PlaylistManagerPage: React.FC = () => {
  const { windows, loading: windowsLoading, refetch: refetchWindows } = useWindows();
  const { media, loading: mediaLoading, refetch: refetchMedia } = useMedia();

  // 0 = no selection yet; useEffect sets it once windows are loaded
  const [selectedWindowId, setSelectedWindowId] = useState<number>(0);

  // Set default window once loaded; also re-validates if the selected window was deleted
  useEffect(() => {
    if (windows.length > 0 && (selectedWindowId === 0 || !windows.some((w) => w.id === selectedWindowId))) {
      setSelectedWindowId(windows[0].id);
    }
  }, [windows, selectedWindowId]);

  return (
    <div className="playlist-manager-page">
      {/* Top Header */}
      <div className="wall-header-bar">
        <div className="wall-header-left">
          <h2 className="wall-title">Playlist & Media Manager</h2>
          <span className="wall-badge" style={{ background: 'rgba(99, 102, 241, 0.15)', color: '#a5b4fc', borderColor: 'rgba(99, 102, 241, 0.3)' }}>
            Content operations
          </span>
        </div>

        <div className="wall-header-right">
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => {
              refetchWindows();
              refetchMedia();
            }}
            disabled={windowsLoading || mediaLoading}
            title="Refresh windows and media library"
          >
            {windowsLoading || mediaLoading ? 'Refreshing...' : 'Refresh'}
          </button>
        </div>
      </div>

      {/* Two-Column Responsive Management Layout */}
      <div className="manager-two-col-layout">
        {/* Column 1: Window Playlist Configuration */}
        <div className="manager-col-playlist">
          <PlaylistEditor
            windows={windows}
            selectedWindowId={selectedWindowId}
            onSelectWindow={setSelectedWindowId}
            mediaList={media}
          />
        </div>

        {/* Column 2: Media Asset Library */}
        <div className="manager-col-media">
          <MediaLibraryManager
            mediaList={media}
            loading={mediaLoading}
            onRefresh={refetchMedia}
          />
        </div>
      </div>
    </div>
  );
};
