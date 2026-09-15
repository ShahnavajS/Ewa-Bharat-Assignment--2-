import React, { useState, useEffect } from 'react';
import { useWindows, useMedia, useWebSocket } from './hooks';
import {
  ConnectionBadge,
  HealthStatusCard,
  WindowListCard,
  MediaLibraryCard,
  LiveSubscriptionCard,
} from './components';
import { DisplayWallPage } from './pages/DisplayWallPage';
import { PlaylistManagerPage } from './pages/PlaylistManagerPage';
import { SyncControlPage } from './pages/SyncControlPage';

export type AppView = 'wall' | 'playlist' | 'sync' | 'inspector';

export const App: React.FC = () => {
  // Navigation state initialized from URL hash or default to 'wall'
  const [activeView, setActiveView] = useState<AppView>(() => {
    const hash = window.location.hash.toLowerCase();
    if (hash.includes('playlist')) return 'playlist';
    if (hash.includes('sync')) return 'sync';
    if (hash.includes('inspector')) return 'inspector';
    return 'wall';
  });

  // Sync state with URL hash
  useEffect(() => {
    const handleHashChange = () => {
      const hash = window.location.hash.toLowerCase();
      if (hash.includes('playlist')) {
        setActiveView('playlist');
      } else if (hash.includes('sync')) {
        setActiveView('sync');
      } else if (hash.includes('inspector')) {
        setActiveView('inspector');
      } else {
        setActiveView('wall');
      }
    };
    window.addEventListener('hashchange', handleHashChange);
    return () => window.removeEventListener('hashchange', handleHashChange);
  }, []);

  const switchView = (view: AppView) => {
    setActiveView(view);
    if (view === 'wall') {
      window.location.hash = 'display';
    } else if (view === 'playlist') {
      window.location.hash = 'playlist';
    } else if (view === 'sync') {
      window.location.hash = 'sync';
    } else {
      window.location.hash = 'inspector';
    }
  };

  // State for Inspector view
  const { windows, loading: windowsLoading, error: windowsError, refetch: refetchWindows } = useWindows();
  const { media, loading: mediaLoading, error: mediaError, refetch: refetchMedia } = useMedia();
  const [selectedWindowId, setSelectedWindowId] = useState<number | null>(null);

  const {
    status: wsStatus,
    snapshot,
    subscribedWindowId,
    messageLog,
    subscribe,
    connect,
    disconnect,
  } = useWebSocket();

  useEffect(() => {
    if (windows.length > 0 && selectedWindowId === null) {
      const firstId = windows[0].id;
      setSelectedWindowId(firstId);
      subscribe(firstId);
    }
  }, [windows, selectedWindowId, subscribe]);

  const handleSelectWindow = (id: number) => {
    setSelectedWindowId(id);
    subscribe(id);
  };

  const handleResubscribe = () => {
    if (selectedWindowId !== null) {
      subscribe(selectedWindowId);
    }
  };

  const activeError = windowsError?.message || mediaError?.message;

  return (
    <div className="app-container">
      {/* Global Navigation Header */}
      <header className="app-header">
        <div className="brand-group">
          <div className="brand-title">
            <span>EVA Bharat</span>
            <span className="phase-pill">Media operations</span>
          </div>
          <div className="brand-subtitle">
            Multi-Window Media Sequencer with Synchronized Playback
          </div>
        </div>

        {/* View Switcher Tabs */}
        <nav className="nav-tabs">
          <button
            className={`nav-tab-btn ${activeView === 'wall' ? 'active' : ''}`}
            onClick={() => switchView('wall')}
          >
            Display wall
          </button>
          <button
            className={`nav-tab-btn ${activeView === 'playlist' ? 'active' : ''}`}
            onClick={() => switchView('playlist')}
          >
            Playlists
          </button>
          <button
            className={`nav-tab-btn ${activeView === 'sync' ? 'active' : ''}`}
            onClick={() => switchView('sync')}
          >
            Sync control
          </button>
          <button
            className={`nav-tab-btn ${activeView === 'inspector' ? 'active' : ''}`}
            onClick={() => switchView('inspector')}
          >
            System status
          </button>
        </nav>

        <div className="header-meta">
          <ConnectionBadge status={wsStatus} />
        </div>
      </header>

      {/* Main View Switcher */}
      {activeView === 'wall' ? (
        <DisplayWallPage />
      ) : activeView === 'playlist' ? (
        <PlaylistManagerPage />
      ) : activeView === 'sync' ? (
        <SyncControlPage />
      ) : (
        <>
          {/* Inspector Error Banner */}
          {activeError && (
            <div className="error-banner">
              <div>
                <strong>Backend Connection Issue:</strong> {activeError}
              </div>
              <button
                className="btn btn-secondary"
                onClick={() => {
                  refetchWindows();
                  refetchMedia();
                }}
                style={{ fontSize: '0.75rem', padding: '3px 8px' }}
              >
                Retry
              </button>
            </div>
          )}

          {/* Inspector Dashboard Grid */}
          <main className="dashboard-grid">
            <HealthStatusCard />
            <LiveSubscriptionCard
              status={wsStatus}
              subscribedWindowId={subscribedWindowId}
              snapshot={snapshot}
              messageLog={messageLog}
              onConnect={connect}
              onDisconnect={disconnect}
              onResubscribe={handleResubscribe}
            />
            <WindowListCard
              windows={windows}
              loading={windowsLoading}
              selectedWindowId={selectedWindowId}
              onSelectWindow={handleSelectWindow}
              onRefresh={refetchWindows}
            />
            <MediaLibraryCard
              media={media}
              loading={mediaLoading}
              onRefresh={refetchMedia}
            />
          </main>
        </>
      )}
    </div>
  );
};

export default App;
