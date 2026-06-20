import { useEffect, useRef } from 'react';
import { useProxyStore } from '../store/proxyStore';
import { useConnectionStore } from '../store/connectionStore';
import { getToken } from '../lib/auth';
import type { WsFrame } from '../lib/types';

function trafficUrl(): string {
  const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
  // Browsers cannot set headers on a WebSocket handshake, so the auth token (if
  // any) is passed as a query parameter, which the server accepts for /ws/.
  const token = getToken();
  const query = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${scheme}://${location.host}/ws/traffic${query}`;
}

function isWsFrame(value: unknown): value is WsFrame {
  if (typeof value !== 'object' || value === null) return false;
  const t = (value as { type?: unknown }).type;
  return (t === 'traffic' || t === 'intercept') && 'data' in value;
}

/**
 * Connects once to the live traffic stream and dispatches frames into the
 * proxy store. Reconnects automatically with capped exponential backoff.
 */
export function useWebSocket(): void {
  const applyTraffic = useProxyStore((s) => s.applyTrafficFrame);
  const applyIntercept = useProxyStore((s) => s.applyInterceptFrame);
  const setStatus = useConnectionStore((s) => s.setStatus);

  // Refs avoid re-running the effect when store actions change identity.
  const applyTrafficRef = useRef(applyTraffic);
  const applyInterceptRef = useRef(applyIntercept);
  applyTrafficRef.current = applyTraffic;
  applyInterceptRef.current = applyIntercept;

  useEffect(() => {
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;
    let disposed = false;

    const connect = () => {
      if (disposed) return;
      setStatus('connecting');
      socket = new WebSocket(trafficUrl());

      socket.onopen = () => {
        attempt = 0;
        setStatus('open');
      };

      socket.onmessage = (event) => {
        try {
          const parsed: unknown = JSON.parse(event.data as string);
          if (!isWsFrame(parsed)) return;
          if (parsed.type === 'traffic') {
            applyTrafficRef.current(parsed.data);
          } else {
            applyInterceptRef.current(parsed.data);
          }
        } catch {
          // Ignore malformed frames.
        }
      };

      socket.onclose = () => {
        setStatus('closed');
        if (disposed) return;
        // Capped exponential backoff: 0.5s, 1s, 2s, ... up to 10s.
        const delay = Math.min(10_000, 500 * 2 ** attempt);
        attempt += 1;
        reconnectTimer = setTimeout(connect, delay);
      };

      socket.onerror = () => {
        // Close handler drives the reconnect; just force-close here.
        socket?.close();
      };
    };

    connect();

    return () => {
      disposed = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (socket) {
        socket.onclose = null;
        socket.close();
      }
    };
  }, [setStatus]);
}
