import React from 'react';
import { Window } from '../types';

interface WindowListCardProps {
  windows: Window[];
  loading: boolean;
  selectedWindowId: number | null;
  onSelectWindow: (id: number) => void;
  onRefresh: () => void;
}

export const WindowListCard: React.FC<WindowListCardProps> = ({
  windows,
  loading,
  selectedWindowId,
  onSelectWindow,
  onRefresh,
}) => {
  return (
    <div className="card" style={{ gridColumn: 'span 6' }}>
      <div className="card-title">
        <span>Configured Display Windows ({windows.length})</span>
        <button
          className="btn btn-secondary"
          onClick={onRefresh}
          disabled={loading}
          style={{ fontSize: '0.75rem', padding: '3px 8px' }}
        >
          {loading ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {loading && windows.length === 0 ? (
        <div style={{ color: 'var(--text-muted)', fontSize: '0.9rem', padding: '16px 0' }}>
          Loading windows from backend...
        </div>
      ) : windows.length === 0 ? (
        <div style={{ color: 'var(--text-muted)', fontSize: '0.9rem', padding: '16px 0' }}>
          No windows configured. Run database seed migration to populate default displays.
        </div>
      ) : (
        <div className="window-list">
          {windows.map((win) => {
            const isSelected = selectedWindowId === win.id;
            return (
              <div
                key={win.id}
                className={`window-item ${isSelected ? 'active' : ''}`}
                onClick={() => onSelectWindow(win.id)}
                title="Click to subscribe via WebSocket"
              >
                <div className="window-info">
                  <h4>{win.name} {isSelected && <span style={{ color: 'var(--accent-primary)', fontSize: '0.75rem' }}>● SUBSCRIBED</span>}</h4>
                  <p>{win.description || 'Display window'}</p>
                </div>
                <div style={{ textAlign: 'right' }}>
                  <div style={{ fontSize: '0.8rem', fontWeight: 600 }}>
                    {win.cycle_duration_seconds}s cycle
                  </div>
                  <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>
                    ID #{win.id}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
