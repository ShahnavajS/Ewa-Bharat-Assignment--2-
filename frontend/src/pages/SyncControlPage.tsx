import React, { useState, useEffect, useCallback, useRef } from 'react';
import { wsClient } from '../websocket/client';
import { startSync, getSyncState, cancelSync } from '../api/sync';
import { getMedia } from '../api/media';
import { MediaItem, ApiError } from '../types';
import { ActiveSyncState, StartSyncInput } from '../types/sync';
import { ServerWebSocketMessage } from '../types/websocket';

function fmtDuration(secs: number): string {
  if (secs <= 0) return '0s';
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

function fmtTimestamp(iso: string | undefined): string {
  if (!iso) return '-';
  try { return new Date(iso).toLocaleTimeString(); } catch { return '-'; }
}

interface StatusBannerProps { syncState: ActiveSyncState | null; remaining: number; }

const StatusBanner: React.FC<StatusBannerProps> = ({ syncState, remaining }) => {
  if (!syncState?.active) {
    return (
      <div className="sync-status-banner sync-idle">
        <div className="sync-status-icon">IDLE</div>
        <div className="sync-status-text">
          <span className="sync-status-label">IDLE</span>
          <span className="sync-status-detail">No synchronized playback active</span>
        </div>
      </div>
    );
  }
  const total = syncState.duration_seconds ?? 1;
  const pct = Math.max(0, Math.min(1, remaining / total));
  const circ = 2 * Math.PI * 20;
  return (
    <div className="sync-status-banner sync-active">
      <div className="sync-status-icon sync-pulse">LIVE</div>
      <div className="sync-status-text">
        <span className="sync-status-label">SYNC ACTIVE</span>
        <span className="sync-status-detail">
          {syncState.media?.title ?? `Media #${syncState.media?.id}`}{' / '}
          <strong>{fmtDuration(remaining)}</strong> remaining
        </span>
      </div>
      <div className="sync-progress-ring">
        <svg viewBox="0 0 48 48" width="48" height="48">
          <circle cx="24" cy="24" r="20" fill="none" stroke="rgba(255,255,255,0.15)" strokeWidth="4" />
          <circle cx="24" cy="24" r="20" fill="none" stroke="#2457d6" strokeWidth="4"
            strokeDasharray={String(circ)}
            strokeDashoffset={String(circ * (1 - pct))}
            strokeLinecap="round"
            style={{ transition: 'stroke-dashoffset 1s linear', transformOrigin: 'center', transform: 'rotate(-90deg)' }} />
        </svg>
        <span className="sync-ring-label">{remaining}s</span>
      </div>
    </div>
  );
};

interface ActiveSyncInfoProps { syncState: ActiveSyncState; }
const ActiveSyncInfo: React.FC<ActiveSyncInfoProps> = ({ syncState }) => (
  <div className="sync-info-grid">
    <div className="sync-info-item"><span className="sync-info-key">Media</span><span className="sync-info-val">{syncState.media?.title ?? `ID ${syncState.media?.id}`}</span></div>
    <div className="sync-info-item"><span className="sync-info-key">Duration</span><span className="sync-info-val">{fmtDuration(syncState.duration_seconds ?? 0)}</span></div>
    <div className="sync-info-item"><span className="sync-info-key">Started</span><span className="sync-info-val">{fmtTimestamp(syncState.started_at)}</span></div>
    <div className="sync-info-item"><span className="sync-info-key">Ends At</span><span className="sync-info-val">{fmtTimestamp(syncState.ends_at)}</span></div>
    {syncState.sync_id != null && (<div className="sync-info-item"><span className="sync-info-key">Sync ID</span><span className="sync-info-val">#{syncState.sync_id}</span></div>)}
  </div>
);

export const SyncControlPage: React.FC = () => {
  const [mediaList, setMediaList] = useState<MediaItem[]>([]);
  const [mediaLoading, setMediaLoading] = useState(true);
  const [selectedMediaId, setSelectedMediaId] = useState<number | ''>('');
  const [durationInput, setDurationInput] = useState<string>('30');
  const [triggeredBy, setTriggeredBy] = useState<string>('operator');
  const [syncState, setSyncState] = useState<ActiveSyncState | null>(null);
  const [syncLoading, setSyncLoading] = useState(true);
  const [remaining, setRemaining] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const [wsEvents, setWsEvents] = useState<{ type: string; time: string }[]>([]);
  const countdownRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    getMedia().then((items) => setMediaList(items ?? [])).catch(() => setMediaList([])).finally(() => setMediaLoading(false));
  }, []);

  const refreshSyncState = useCallback(async () => {
    try {
      const state = await getSyncState();
      setSyncState(state);
      if (state.active && state.remaining_seconds != null) setRemaining(state.remaining_seconds);
    } catch { /* ignore */ } finally { setSyncLoading(false); }
  }, []);

  useEffect(() => { refreshSyncState(); }, [refreshSyncState]);

  useEffect(() => {
    if (countdownRef.current) { clearInterval(countdownRef.current); countdownRef.current = null; }
    if (syncState?.active) {
      // Start a countdown interval. Do NOT put `remaining` in the deps array —
      // it decrements every second, which would restart the interval on each tick.
      // Instead use functional setRemaining to read the previous value without a closure dep.
      countdownRef.current = setInterval(() => {
        setRemaining((prev) => {
          if (prev <= 1) {
            clearInterval(countdownRef.current!);
            countdownRef.current = null;
            // Reconcile with backend once local countdown reaches zero
            setTimeout(refreshSyncState, 500);
            return 0;
          }
          return prev - 1;
        });
      }, 1000);
    }
    return () => { if (countdownRef.current) { clearInterval(countdownRef.current); countdownRef.current = null; } };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [syncState?.active, refreshSyncState]);

  useEffect(() => {
    const unsub = wsClient.onMessage((msg: ServerWebSocketMessage) => {
      if (msg.type === 'SYNC_STARTED' || msg.type === 'SYNC_ENDED') {
        setWsEvents((prev) => [{ type: msg.type, time: new Date().toLocaleTimeString() }, ...prev].slice(0, 20));
        setTimeout(refreshSyncState, 200);
      }
    });
    return unsub;
  }, [refreshSyncState]);

  useEffect(() => {
    if (actionSuccess || actionError) {
      const t = setTimeout(() => { setActionSuccess(null); setActionError(null); }, 4000);
      return () => clearTimeout(t);
    }
  }, [actionSuccess, actionError]);

  const handleStartSync = useCallback(async () => {
    if (submitting) return;
    setActionError(null); setActionSuccess(null);
    const mediaId = Number(selectedMediaId);
    const duration = Number(durationInput);
    if (!mediaId || isNaN(mediaId)) { setActionError('Please select a media asset.'); return; }
    if (!duration || isNaN(duration) || duration < 1 || duration > 18000) { setActionError('Duration must be between 1 second and 5 hours.'); return; }
    const payload: StartSyncInput = { media_id: mediaId, duration_seconds: duration, triggered_by: triggeredBy.trim() || 'operator' };
    setSubmitting(true);
    try {
      await startSync(payload);
      setActionSuccess('Synchronized playback started across all windows.');
      await refreshSyncState();
    } catch (err: unknown) {
      setActionError(err instanceof ApiError ? `Failed to start sync: ${err.message}` : 'Failed to start sync. Is the backend reachable?');
    } finally { setSubmitting(false); }
  }, [submitting, selectedMediaId, durationInput, triggeredBy, refreshSyncState]);

  const handleCancelSync = useCallback(async () => {
    if (cancelling) return;
    setActionError(null); setActionSuccess(null);
    setCancelling(true);
    try {
      await cancelSync();
      setActionSuccess('Synchronized playback cancelled.');
      await refreshSyncState();
    } catch (err: unknown) {
      setActionError(err instanceof ApiError ? `Failed to cancel: ${err.message}` : 'Failed to cancel sync.');
    } finally { setCancelling(false); }
  }, [cancelling, refreshSyncState]);

  const isActive = syncState?.active === true;
  const durationValid = Number(durationInput) >= 1 && Number(durationInput) <= 18000;
  const canStart = !isActive && !!selectedMediaId && durationValid && !submitting;

  return (
    <div className="sync-page">
      <div className="sync-page-header">
        <h1 className="sync-page-title">Sync control</h1>
        <p className="sync-page-subtitle">Trigger synchronized override playback across all display windows simultaneously.</p>
      </div>

      {actionError && (
        <div className="sync-alert sync-alert-error" role="alert">
          <span>{actionError}</span>
          <button className="sync-alert-close" onClick={() => setActionError(null)}>Close</button>
        </div>
      )}
      {actionSuccess && (
        <div className="sync-alert sync-alert-success" role="alert">
          <span>{actionSuccess}</span>
          <button className="sync-alert-close" onClick={() => setActionSuccess(null)}>Close</button>
        </div>
      )}

      <div className="sync-layout">
        <div className="sync-main-col">
          {syncLoading ? (
            <div className="sync-status-banner sync-idle">
              <div className="sync-status-icon">WAIT</div>
              <div className="sync-status-text"><span className="sync-status-label">CHECKING...</span><span className="sync-status-detail">Fetching current sync state</span></div>
            </div>
          ) : (
            <StatusBanner syncState={syncState} remaining={remaining} />
          )}

          {isActive && syncState && (
            <div className="sync-card">
              <div className="sync-card-header"><span className="sync-card-title">Active Sync Details</span></div>
              <div className="sync-card-body"><ActiveSyncInfo syncState={syncState} /></div>
            </div>
          )}

          <div className="sync-card">
            <div className="sync-card-header">
              <span className="sync-card-title">Launch Synchronized Playback</span>
              {isActive && <span className="sync-card-badge sync-card-badge-active">Sync in progress</span>}
            </div>
            <div className="sync-card-body">
              {isActive ? (
                <>
                  <p className="sync-form-hint">A sync session is already active. Cancel it before launching a new one.</p>
                  <button id="sync-cancel-btn" className="btn btn-sync-cancel" onClick={handleCancelSync} disabled={cancelling} aria-disabled={cancelling}>
                    {cancelling ? <><span className="btn-spinner" />Cancelling...</> : <>Cancel sync</>}
                  </button>
                </>
              ) : (
                <form className="sync-form" onSubmit={(e) => { e.preventDefault(); handleStartSync(); }}>
                  <div className="sync-form-group">
                    <label className="sync-form-label" htmlFor="sync-media-select">Media Asset</label>
                    {mediaLoading ? (
                      <div className="sync-form-loading">Loading media library...</div>
                    ) : mediaList.length === 0 ? (
                      <div className="sync-form-empty">No media assets found. Register media first.</div>
                    ) : (
                      <select id="sync-media-select" className="sync-form-select" value={selectedMediaId}
                        onChange={(e) => setSelectedMediaId(e.target.value === '' ? '' : Number(e.target.value))} disabled={submitting}>
                        <option value="">Select a media asset</option>
                        {mediaList.map((m) => (<option key={m.id} value={m.id}>{m.title} ({m.type})</option>))}
                      </select>
                    )}
                  </div>
                  <div className="sync-form-group">
                    <label className="sync-form-label" htmlFor="sync-duration-input">Duration (seconds)</label>
                    <div className="sync-duration-row">
                      <input id="sync-duration-input" className="sync-form-input" type="number" min={1} max={18000}
                        value={durationInput} onChange={(e) => setDurationInput(e.target.value)} disabled={submitting} placeholder="30" />
                      {durationInput && !isNaN(Number(durationInput)) && Number(durationInput) > 0 && (
                        <span className="sync-duration-hint">{fmtDuration(Number(durationInput))}</span>
                      )}
                    </div>
                    <span className="sync-form-hint">Between 1 second and 5 hours</span>
                  </div>
                  <div className="sync-form-group">
                    <label className="sync-form-label" htmlFor="sync-triggered-by">Triggered By <span className="sync-form-optional">(optional)</span></label>
                    <input id="sync-triggered-by" className="sync-form-input" type="text" maxLength={64}
                      value={triggeredBy} onChange={(e) => setTriggeredBy(e.target.value)} disabled={submitting} placeholder="operator" />
                  </div>
                  <div className="sync-presets">
                    <span className="sync-presets-label">Quick:</span>
                    {[10, 30, 60, 120, 300].map((secs) => (
                      <button key={secs} type="button" className={`sync-preset-btn ${durationInput === String(secs) ? 'active' : ''}`}
                        onClick={() => setDurationInput(String(secs))} disabled={submitting}>{fmtDuration(secs)}</button>
                    ))}
                  </div>
                  <button id="sync-start-btn" type="submit" className="btn btn-sync-start" disabled={!canStart} aria-disabled={!canStart}>
                    {submitting ? <><span className="btn-spinner" />Starting...</> : <>Start synchronized playback</>}
                  </button>
                </form>
              )}
            </div>
          </div>
        </div>

        <div className="sync-side-col">
          <div className="sync-card">
            <div className="sync-card-header">
              <span className="sync-card-title">Live Events</span>
              {wsEvents.length > 0 && <button className="sync-card-clear" onClick={() => setWsEvents([])}>Clear</button>}
            </div>
            <div className="sync-card-body">
              {wsEvents.length === 0 ? (
                <div className="sync-event-empty">Waiting for sync events...</div>
              ) : (
                <ul className="sync-event-list">
                  {wsEvents.map((ev, i) => (
                    <li key={i} className={`sync-event-item ${ev.type === 'SYNC_STARTED' ? 'event-started' : 'event-ended'}`}>
                      <span className="sync-event-badge">{ev.type === 'SYNC_STARTED' ? 'Started' : 'Ended'}</span>
                      <span className="sync-event-time">{ev.time}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>

          <div className="sync-card sync-card-info">
            <div className="sync-card-header"><span className="sync-card-title">How It Works</span></div>
            <div className="sync-card-body">
              <ol className="sync-how-list">
                <li>Select the media asset to broadcast.</li>
                <li>Set the override duration.</li>
                <li>Click <strong>Start Sync Playback</strong>.</li>
                <li>All display windows switch to SYNC mode simultaneously via WebSocket.</li>
                <li>After the duration expires (or you cancel), windows resume their normal playlists.</li>
              </ol>
              <div className="sync-info-note">
                <strong>Note:</strong> The backend PlaybackEngine is the authoritative source. The Display Wall reacts to the same <code>SYNC_STARTED</code> / <code>SYNC_ENDED</code> events independently.
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SyncControlPage;
