import React, { useState, useEffect, useCallback } from 'react';
import { apiClient } from '../api/client';
import { HealthStatus, ApiError } from '../types';
import { useServerTime } from '../hooks/useServerTime';

export const HealthStatusCard: React.FC = () => {
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const { isoString, offsetMs } = useServerTime(500);

  const checkHealth = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiClient.getHealth();
      setHealth(data);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(`${err.code}: ${err.message}`);
      } else {
        setError('Failed to reach backend REST API');
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    checkHealth();
    // Periodically re-check health every 15s
    const interval = setInterval(checkHealth, 15000);
    return () => clearInterval(interval);
  }, [checkHealth]);

  return (
    <div className="card" style={{ gridColumn: 'span 6' }}>
      <div className="card-title">
        <span>Backend System & Server Time</span>
        <button
          className="btn btn-secondary"
          onClick={checkHealth}
          disabled={loading}
          style={{ fontSize: '0.75rem', padding: '3px 8px' }}
        >
          {loading ? 'Checking...' : 'Refresh'}
        </button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginTop: '10px' }}>
        <div>
          <div className="stat-desc">REST API Status</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '4px' }}>
            <span
              className={`status-dot ${health?.status === 'ok' ? 'connected' : 'error'}`}
            />
            <span style={{ fontWeight: 600, fontSize: '1.1rem' }}>
              {error ? 'Degraded / Offline' : health?.status === 'ok' ? 'Online' : 'Checking...'}
            </span>
          </div>
          <div className="stat-desc" style={{ marginTop: '6px' }}>
            Database: <span style={{ color: health?.database === 'connected' ? 'var(--status-connected)' : 'var(--status-error)' }}>
              {health?.database || 'unknown'}
            </span>
          </div>
        </div>

        <div>
          <div className="stat-desc">Synchronized Server Time (UTC)</div>
          <div
            style={{
              fontFamily: 'var(--font-mono)',
              fontSize: '0.95rem',
              fontWeight: 600,
              color: 'var(--accent-primary)',
              marginTop: '4px',
            }}
          >
            {isoString.replace('T', ' ').replace('Z', ' UTC')}
          </div>
          <div className="stat-desc" style={{ marginTop: '6px' }}>
            Clock Offset: <span className="code-snippet">{offsetMs >= 0 ? `+${offsetMs}ms` : `${offsetMs}ms`}</span>
          </div>
        </div>
      </div>

      {error && (
        <div style={{ marginTop: '12px', fontSize: '0.8rem', color: '#f87171' }}>
          {error}
        </div>
      )}
    </div>
  );
};
