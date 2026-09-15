import React, { useState, useRef } from 'react';
import { useWindowPlayback } from '../hooks/useWindowPlayback';
import { MediaPlayer } from './MediaPlayer';

interface DisplayWindowProps {
  windowId: number;
  windowName: string;
}

export const DisplayWindow: React.FC<DisplayWindowProps> = ({
  windowId,
  windowName,
}) => {
  const {
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
  } = useWindowPlayback(windowId);

  const containerRef = useRef<HTMLDivElement | null>(null);
  const [isFullscreen, setIsFullscreen] = useState<boolean>(false);
  const [isMuted, setIsMuted] = useState<boolean>(true);

  const toggleFullscreen = () => {
    if (!containerRef.current) return;

    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen().then(() => {
        setIsFullscreen(true);
      }).catch(() => {
        // Fullscreen request rejected
      });
    } else {
      document.exitFullscreen().then(() => {
        setIsFullscreen(false);
      }).catch(() => {});
    }
  };

  // Determine active media & mode:
  // In sync mode, sync media takes precedence
  const effectiveMedia = isSyncActive
    ? syncMedia || playbackState?.media || null
    : playbackState?.media || null;

  const effectiveMode = isSyncActive
    ? 'sync'
    : (playbackState?.mode || (playbackState?.playlist_empty ? 'inactive' : 'normal'));

  const itemDuration = (localElapsedSeconds + localRemainingSeconds) || 1;
  const progressPercent = Math.min(Math.round((localElapsedSeconds / itemDuration) * 100), 100);

  return (
    <div
      ref={containerRef}
      className={`display-window-card ${isFullscreen ? 'fullscreen-mode' : ''} ${isSyncActive ? 'sync-active' : ''}`}
      id={`display-window-${windowId}`}
    >
      {/* Top Header Overlay */}
      <div className="window-header-overlay">
        <div className="window-title-block">
          <span className={`status-dot ${connectionStatus}`} title={`Status: ${connectionStatus}`} />
          <h3 className="window-name-text">
            {windowDetails?.name || windowName}
          </h3>
          <span className="window-id-pill">ID #{windowId}</span>
        </div>

        <div className="window-controls-block">
          {/* Playback Mode Badge */}
          <span className={`mode-badge ${loading ? '' : effectiveMode}`}>
            {loading ? 'LOADING' : effectiveMode === 'sync' ? 'SYNC OVERRIDE' : effectiveMode.toUpperCase()}
          </span>

          {/* Mute/Unmute Audio Toggle */}
          {effectiveMedia?.type === 'video' && (
            <button
              className="control-icon-btn"
              onClick={() => setIsMuted(!isMuted)}
              title={isMuted ? 'Unmute Audio' : 'Mute Audio'}
              aria-label={isMuted ? 'Unmute Audio' : 'Mute Audio'}
            >
              {isMuted ? 'Muted' : 'Audio'}
            </button>
          )}

          {/* Fullscreen Toggle */}
          <button
            className="control-icon-btn"
            onClick={toggleFullscreen}
            title={isFullscreen ? 'Exit Fullscreen' : 'Enter Fullscreen'}
            aria-label={isFullscreen ? 'Exit Fullscreen' : 'Enter Fullscreen'}
          >
            {isFullscreen ? 'Exit' : 'Expand'}
          </button>
        </div>
      </div>

      {/* Media Playback Viewport */}
      <div className="window-media-viewport">
        {loading ? (
          <div className="player-fallback loading">
            <div className="fallback-icon">LOADING</div>
            <div className="fallback-title">Preparing playback</div>
            <div className="fallback-desc">Fetching the current server position...</div>
          </div>
        ) : error ? (
          <div className="player-fallback error">
            <div className="fallback-icon">ERROR</div>
            <div className="fallback-title">Playback Error</div>
            <div className="fallback-desc">{error}</div>
          </div>
        ) : (
          <MediaPlayer
            media={effectiveMedia}
            mode={effectiveMode}
            elapsedSeconds={localElapsedSeconds}
            remainingSeconds={localRemainingSeconds}
            isMuted={isMuted}
          />
        )}
      </div>

      {/* Bottom Progress & Item Info Bar */}
      <div className="window-footer-overlay">
        <div className="progress-bar-track">
          <div
            className={`progress-bar-fill ${effectiveMode === 'sync' ? 'sync' : ''}`}
            style={{ width: `${progressPercent}%` }}
          />
        </div>

        <div className="window-meta-row">
          <div className="item-meta-left">
            {loading ? (
              <span className="item-title-text text-muted">Loading current item</span>
            ) : effectiveMedia ? (
              <>
                <span className={`type-pill ${effectiveMedia.type}`}>
                  {effectiveMedia.type}
                </span>
                <span className="item-title-text" title={effectiveMedia.title}>
                  {effectiveMedia.title}
                </span>
                {!isSyncActive && playbackState?.position && (
                  <span className="item-position-text">
                    ({playbackState.position}/{playlist.length})
                  </span>
                )}
              </>
            ) : (
              <span className="item-title-text text-muted">No active media</span>
            )}
          </div>

          <div className="item-meta-right">
            {effectiveMedia && (
              <span className="timer-text font-mono">
                {Math.ceil(localRemainingSeconds)}s remaining
              </span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
