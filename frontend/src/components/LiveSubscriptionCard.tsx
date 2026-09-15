import React from 'react';
import {
  ConnectionStatus,
  WindowPlaybackResponse,
  ServerWebSocketMessage,
} from '../types';

interface LiveSubscriptionCardProps {
  status: ConnectionStatus;
  subscribedWindowId: number | null;
  snapshot: WindowPlaybackResponse | null;
  messageLog: ServerWebSocketMessage[];
  onConnect: () => void;
  onDisconnect: () => void;
  onResubscribe: () => void;
}

export const LiveSubscriptionCard: React.FC<LiveSubscriptionCardProps> = ({
  status,
  subscribedWindowId,
  snapshot,
  messageLog,
  onConnect,
  onDisconnect,
  onResubscribe,
}) => {
  const currentPlayback = snapshot?.current_playback_state;

  return (
    <div className="card" style={{ gridColumn: 'span 6' }}>
      <div className="card-title">
        <span>Realtime Hub Subscription Inspector</span>
        <div style={{ display: 'flex', gap: '6px' }}>
          {status === 'connected' ? (
            <>
              <button
                className="btn btn-secondary"
                onClick={onResubscribe}
                style={{ fontSize: '0.75rem', padding: '3px 8px' }}
                disabled={!subscribedWindowId}
              >
                Resend Sub
              </button>
              <button
                className="btn btn-danger"
                onClick={onDisconnect}
                style={{ fontSize: '0.75rem', padding: '3px 8px' }}
              >
                Disconnect
              </button>
            </>
          ) : (
            <button
              className="btn btn-primary"
              onClick={onConnect}
              style={{ fontSize: '0.75rem', padding: '3px 8px' }}
            >
              Connect
            </button>
          )}
        </div>
      </div>

      <div style={{ marginBottom: '16px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div className="stat-desc">Subscribed Window</div>
          <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>
            {subscribedWindowId ? `Window #${subscribedWindowId} (${snapshot?.name || 'Loading...'})` : 'None (Select a window)'}
          </div>
        </div>

        {snapshot && currentPlayback && (
          <div
            style={{
              background: 'rgba(255, 255, 255, 0.02)',
              border: '1px solid var(--border-color)',
              borderRadius: 'var(--radius-md)',
              padding: '12px',
              marginTop: '10px',
            }}
          >
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px' }}>
              <span className="stat-desc">Playback Mode:</span>
              <span
                style={{
                  fontWeight: 600,
                  color: currentPlayback.mode === 'sync' ? 'var(--accent-primary)' : 'var(--success)',
                  textTransform: 'uppercase',
                  fontSize: '0.85rem',
                }}
              >
                {currentPlayback.mode}
              </span>
            </div>

            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '6px', fontSize: '0.85rem' }}>
              <span className="stat-desc">Active Item:</span>
              <span>
                {currentPlayback.media?.title || (currentPlayback.playlist_empty ? 'Playlist Empty' : 'None')}
              </span>
            </div>

            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
              <span className="stat-desc">Position & Progress:</span>
              <span className="code-snippet">
                {currentPlayback.elapsed_seconds ?? 0}s / {(currentPlayback.elapsed_seconds ?? 0) + (currentPlayback.remaining_seconds ?? 0)}s (Item {currentPlayback.position ?? 0}/{snapshot.playlist?.length ?? 0})
              </span>
            </div>
          </div>
        )}
      </div>

      <div className="stat-desc" style={{ marginBottom: '6px' }}>
        Recent WebSocket Frames ({messageLog.length})
      </div>
      <div className="terminal-box">
        {messageLog.length === 0 ? (
          <div style={{ color: 'var(--text-muted)' }}>Waiting for inbound frames from hub...</div>
        ) : (
          messageLog.slice(0, 10).map((msg, idx) => {
            const timeStr = msg.server_time ? new Date(msg.server_time).toLocaleTimeString() : 'now';
            return (
              <div key={idx} className="log-entry">
                <span className="log-time">[{timeStr}]</span>
                <span className="log-type">{msg.type}</span>
                <span style={{ color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {msg.type === 'TIME_SYNC'
                    ? 'periodic time sync'
                    : msg.type === 'STATE_SNAPSHOT'
                    ? `window #${msg.window_id} snapshot received`
                    : msg.type === 'PLAYLIST_UPDATED'
                    ? `playlist updated for window #${msg.window_id}`
                    : msg.type === 'SYNC_STARTED'
                    ? 'global synchronized playback override started'
                    : msg.type === 'SYNC_ENDED'
                    ? 'global sync ended, normal resumed'
                    : JSON.stringify(msg).slice(0, 50)}
                </span>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
};
