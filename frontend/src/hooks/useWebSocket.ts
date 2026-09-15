import { useState, useEffect, useCallback } from 'react';
import { wsClient } from '../websocket/client';
import {
  ConnectionStatus,
  ServerWebSocketMessage,
  WindowPlaybackResponse,
} from '../types';
import { isStateSnapshot } from '../websocket/protocol';

export function useWebSocket(initialWindowId?: number) {
  const [status, setStatus] = useState<ConnectionStatus>(() => wsClient.getStatus());
  const [lastMessage, setLastMessage] = useState<ServerWebSocketMessage | null>(null);
  const [snapshot, setSnapshot] = useState<WindowPlaybackResponse | null>(null);
  const [subscribedWindowId, setSubscribedWindowId] = useState<number | null>(
    () => initialWindowId ?? wsClient.getSubscribedWindowId()
  );
  const [messageLog, setMessageLog] = useState<ServerWebSocketMessage[]>([]);

  useEffect(() => {
    // Subscribe to status changes
    const unsubStatus = wsClient.onStatusChange((newStatus) => {
      setStatus(newStatus);
    });

    // Subscribe to incoming messages
    const unsubMsg = wsClient.onMessage((msg) => {
      setLastMessage(msg);
      setMessageLog((prev) => [msg, ...prev].slice(0, 50)); // keep last 50 messages

      if (isStateSnapshot(msg)) {
        const payload = msg.playback || msg.data;
        if (payload) {
          setSnapshot(payload);
        }
      }
    });

    // Connect if not already connected
    wsClient.connect();

    if (initialWindowId) {
      wsClient.subscribe(initialWindowId);
      setSubscribedWindowId(initialWindowId);
    }

    return () => {
      unsubStatus();
      unsubMsg();
    };
  }, [initialWindowId]);

  const subscribe = useCallback((windowId: number) => {
    setSubscribedWindowId(windowId);
    wsClient.subscribe(windowId);
  }, []);

  const connect = useCallback(() => {
    wsClient.connect();
  }, []);

  const disconnect = useCallback(() => {
    wsClient.disconnect();
  }, []);

  return {
    status,
    lastMessage,
    snapshot,
    subscribedWindowId,
    messageLog,
    subscribe,
    connect,
    disconnect,
  };
}
