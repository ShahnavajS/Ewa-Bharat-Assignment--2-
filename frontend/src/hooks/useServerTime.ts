import { useState, useEffect } from 'react';
import { wsClient } from '../websocket/client';

/**
 * Hook providing real-time synchronized server time calculated from backend clock offset.
 */
export function useServerTime(refreshIntervalMs: number = 1000) {
  const [serverTime, setServerTime] = useState<Date>(() => wsClient.getServerTime());
  const [offsetMs, setOffsetMs] = useState<number>(() => wsClient.getServerTimeOffsetMs());

  useEffect(() => {
    const timer = setInterval(() => {
      setServerTime(wsClient.getServerTime());
      setOffsetMs(wsClient.getServerTimeOffsetMs());
    }, refreshIntervalMs);

    return () => clearInterval(timer);
  }, [refreshIntervalMs]);

  return {
    serverTime,
    offsetMs,
    isoString: serverTime.toISOString(),
  };
}
