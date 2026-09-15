import React from 'react';
import { useWindows } from '../hooks/useWindows';
import { useServerTime } from '../hooks/useServerTime';
import { DisplayWindow } from '../components/DisplayWindow';

export const DisplayWallPage: React.FC = () => {
  const { windows, loading, error, refetch } = useWindows();
  const { isoString, offsetMs } = useServerTime(500);

  // Compute CSS grid class based on window count
  const getGridClass = () => {
    if (windows.length === 1) return 'grid-single-window';
    if (windows.length === 2) return 'grid-dual-window';
    return 'grid-multi-window';
  };

  return (
    <div className="display-wall-page">
      {/* Wall Header Bar */}
      <div className="wall-header-bar">
        <div className="wall-header-left">
          <h2 className="wall-title">Multi-Window Display Wall</h2>
          <span className="wall-badge">
            {loading ? 'Loading...' : `${windows.length} configured`}
          </span>
        </div>

        <div className="wall-header-right">
          <div className="server-clock-display" title={`Client-Server Offset: ${offsetMs}ms`}>
            <span className="clock-label">UTC CLOCK:</span>
            <span className="clock-val font-mono">
              {isoString.replace('T', ' ').replace('Z', '')}
            </span>
          </div>

          <button
            className="btn btn-secondary btn-sm"
            onClick={refetch}
            disabled={loading}
            title="Reload window list from backend"
          >
            {loading ? 'Refreshing...' : 'Refresh'}
          </button>
        </div>
      </div>

      {/* Error state */}
      {error && (
        <div className="error-banner">
          <div>
            <strong>Failed to connect to backend:</strong> {error.message}
          </div>
          <button className="btn btn-secondary btn-sm" onClick={refetch}>
            Retry
          </button>
        </div>
      )}

      {/* Loading state */}
      {loading && windows.length === 0 && (
        <div className="wall-empty-state">
          <div className="spinner" />
          <p>Connecting to display windows...</p>
        </div>
      )}

      {/* Empty state */}
      {!loading && windows.length === 0 && !error && (
        <div className="wall-empty-state">
          <h3>No Display Windows Found</h3>
          <p>Run the backend seed migration to configure the default display screens.</p>
        </div>
      )}

      {/* Display Wall Grid */}
      {windows.length > 0 && (
        <div className={`display-wall-grid ${getGridClass()}`}>
          {windows.map((win) => (
            <DisplayWindow
              key={win.id}
              windowId={win.id}
              windowName={win.name}
            />
          ))}
        </div>
      )}
    </div>
  );
};
