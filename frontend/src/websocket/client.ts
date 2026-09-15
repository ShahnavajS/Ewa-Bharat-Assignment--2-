import {
  ConnectionStatus,
  ServerWebSocketMessage,
  ClientWebSocketMessage,
} from '../types/websocket';
import { createSubscribeMessage, isTimeSync, isStateSnapshot } from './protocol';
import { ReconnectStrategy, ReconnectConfig } from './reconnect';

export type MessageListener = (message: ServerWebSocketMessage) => void;
export type StatusListener = (status: ConnectionStatus) => void;
export type ErrorListener = (error: Event | Error) => void;

export interface WebSocketClientOptions {
  url?: string;
  autoConnect?: boolean;
  reconnect?: ReconnectConfig;
}

/**
 * Production-ready WebSocket client for EVA Bharat Media Sequencer.
 * Handles bidirectional frames, automatic exponential reconnection, window resubscription,
 * and server clock offset synchronization.
 */
export class SequencerWebSocketClient {
  private readonly url: string;
  private socket: WebSocket | null = null;
  private status: ConnectionStatus = 'disconnected';
  private reconnectStrategy: ReconnectStrategy;
  private reconnectTimer: number | null = null;
  private subscribedWindowId: number | null = null;
  private isExplicitlyClosed: boolean = false;

  // Server clock offset in milliseconds (Server UTC - Local Client UTC)
  private serverTimeOffsetMs: number = 0;

  private messageListeners = new Set<MessageListener>();
  private statusListeners = new Set<StatusListener>();
  private errorListeners = new Set<ErrorListener>();

  constructor(options: WebSocketClientOptions = {}) {
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const defaultWsUrl = import.meta.env.VITE_WS_URL || `${wsProtocol}//${window.location.host}/api/ws`;
    this.url = options.url || defaultWsUrl;
    this.reconnectStrategy = new ReconnectStrategy(options.reconnect);

    if (options.autoConnect) {
      this.connect();
    }
  }

  /**
   * Connect to the WebSocket hub.
   */
  public connect(): void {
    if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
      return; // Already connecting or connected
    }

    this.isExplicitlyClosed = false;
    this.clearReconnectTimer();
    this.setStatus(this.reconnectStrategy.getAttempts() > 0 ? 'reconnecting' : 'connecting');

    try {
      this.socket = new WebSocket(this.url);
      this.bindSocketEvents();
    } catch (err: unknown) {
      const error = err instanceof Error ? err : new Error(String(err));
      this.notifyError(error);
      this.scheduleReconnect();
    }
  }

  /**
   * Subscribe to a specific display window.
   */
  public subscribe(windowId: number): void {
    this.subscribedWindowId = windowId;
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      const msg = createSubscribeMessage(windowId);
      this.send(msg);
    }
  }

  /**
   * Send an outbound message to the server.
   */
  public send(msg: ClientWebSocketMessage): void {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return;
    }
    this.socket.send(JSON.stringify(msg));
  }

  /**
   * Get current connection status.
   */
  public getStatus(): ConnectionStatus {
    return this.status;
  }

  /**
   * Get estimated current server time with clock offset applied.
   */
  public getServerTime(): Date {
    return new Date(Date.now() + this.serverTimeOffsetMs);
  }

  /**
   * Get current server clock offset in milliseconds.
   */
  public getServerTimeOffsetMs(): number {
    return this.serverTimeOffsetMs;
  }

  /**
   * Get currently active subscribed window ID.
   */
  public getSubscribedWindowId(): number | null {
    return this.subscribedWindowId;
  }

  /**
   * Register listener for inbound server messages.
   */
  public onMessage(listener: MessageListener): () => void {
    this.messageListeners.add(listener);
    return () => this.messageListeners.delete(listener);
  }

  /**
   * Register listener for connection status changes.
   */
  public onStatusChange(listener: StatusListener): () => void {
    this.statusListeners.add(listener);
    // Immediately emit current status
    listener(this.status);
    return () => this.statusListeners.delete(listener);
  }

  /**
   * Register listener for socket errors.
   */
  public onError(listener: ErrorListener): () => void {
    this.errorListeners.add(listener);
    return () => this.errorListeners.delete(listener);
  }

  /**
   * Cleanly disconnect and tear down socket.
   */
  public disconnect(): void {
    this.isExplicitlyClosed = true;
    this.clearReconnectTimer();

    if (this.socket) {
      // Remove listener to prevent triggering reconnect
      this.socket.onclose = null;
      this.socket.onerror = null;
      this.socket.close();
      this.socket = null;
    }

    this.setStatus('disconnected');
  }

  private bindSocketEvents(): void {
    if (!this.socket) return;

    this.socket.onopen = () => {
      this.reconnectStrategy.reset();
      this.setStatus('connected');

      // If we previously had a window subscription, resubscribe automatically
      if (this.subscribedWindowId !== null) {
        this.subscribe(this.subscribedWindowId);
      }
    };

    this.socket.onmessage = (event: MessageEvent) => {
      try {
        const raw = JSON.parse(event.data) as ServerWebSocketMessage;
        this.updateClockOffset(raw);
        this.notifyMessage(raw);
      } catch {
        // Skip unparseable non-JSON frame
      }
    };

    this.socket.onerror = (event: Event) => {
      this.notifyError(event);
      this.setStatus('error');
    };

    this.socket.onclose = () => {
      this.socket = null;
      if (!this.isExplicitlyClosed) {
        this.setStatus('disconnected');
        this.scheduleReconnect();
      }
    };
  }

  private scheduleReconnect(): void {
    if (this.isExplicitlyClosed || this.reconnectTimer !== null) {
      return;
    }

    const delay = this.reconnectStrategy.getNextDelay();
    this.setStatus('reconnecting');

    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private updateClockOffset(msg: ServerWebSocketMessage): void {
    let serverTimeStr: string | undefined;

    if (isTimeSync(msg)) {
      serverTimeStr = msg.data?.server_time || msg.server_time;
    } else if (isStateSnapshot(msg)) {
      serverTimeStr = msg.server_time;
    } else if ('server_time' in msg && typeof msg.server_time === 'string') {
      serverTimeStr = msg.server_time;
    }

    if (serverTimeStr) {
      const serverMs = new Date(serverTimeStr).getTime();
      if (!isNaN(serverMs)) {
        this.serverTimeOffsetMs = serverMs - Date.now();
      }
    }
  }

  private setStatus(newStatus: ConnectionStatus): void {
    if (this.status !== newStatus) {
      this.status = newStatus;
      this.statusListeners.forEach((listener) => {
        try {
          listener(newStatus);
        } catch {
          // Ignore listener errors
        }
      });
    }
  }

  private notifyMessage(msg: ServerWebSocketMessage): void {
    this.messageListeners.forEach((listener) => {
      try {
        listener(msg);
      } catch {
        // Ignore listener errors
      }
    });
  }

  private notifyError(err: Event | Error): void {
    this.errorListeners.forEach((listener) => {
      try {
        listener(err);
      } catch {
        // Ignore listener errors
      }
    });
  }
}

// Singleton client instance for general app usage
export const wsClient = new SequencerWebSocketClient();
