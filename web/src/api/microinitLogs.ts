import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useWsConnection } from "../hooks/useWsConnection";

export type MicroinitLogStreamState =
  | "idle"
  | "connecting"
  | "live"
  | "error"
  | "closed";

interface WsPayload {
  type: string;
  lines?: string[];
  text?: string;
  error?: string;
}

function resolveMicroinitLogWsUrl(serviceName: string | null): string | null {
  if (!serviceName) return null;
  const path = `/api/v1/admin/microinit/services/${encodeURIComponent(serviceName)}/logs/stream`;
  const base = (import.meta.env.VITE_API_BASE ?? "") as string;
  if (base) {
    const url = new URL(base);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    url.pathname = path;
    url.search = "";
    return url.toString();
  }
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}${path}`;
}

/**
 * Live microinit log stream for one service.
 * On reconnect the buffer is cleared before applying the new history
 * snapshot — avoids duplicating the last N history lines.
 */
export function useMicroinitLogStream(
  serviceName: string | null,
  enabled: boolean,
) {
  const [lines, setLines] = useState<string[]>([]);
  const [state, setState] = useState<MicroinitLogStreamState>("idle");
  const [error, setError] = useState<string | null>(null);
  const pausedRef = useRef(false);
  const [paused, setPaused] = useState(false);

  const url = useMemo(
    () => (enabled ? resolveMicroinitLogWsUrl(serviceName) : null),
    [enabled, serviceName],
  );

  useEffect(() => {
    setLines([]);
    setError(null);
    setState(url ? "connecting" : "idle");
  }, [url]);

  const clear = useCallback(() => {
    setLines([]);
  }, []);

  const setPausedState = useCallback((next: boolean) => {
    pausedRef.current = next;
    setPaused(next);
  }, []);

  useWsConnection({
    url,
    // Server answers {"type":"ping"} with {"type":"pong"} (see StreamLogs).
    pingIntervalMs: 10_000,
    pongTimeoutMs: 30_000,
    onConnecting: () => {
      setState("connecting");
    },
    onOpen: () => {
      // Clear before history arrives so reconnect does not append duplicates.
      setLines([]);
      setError(null);
      setState("live");
    },
    onClose: () => {
      setState((prev) => (prev === "error" ? prev : "closed"));
    },
    onDispose: () => {
      setState("idle");
    },
    onError: () => {
      setState("error");
      setError("stream_failed");
    },
    onMessage: (data) => {
      let payload: WsPayload;
      try {
        payload = JSON.parse(data) as WsPayload;
      } catch {
        return;
      }
      if (payload.type === "history") {
        if (pausedRef.current) return;
        setLines(payload.lines ?? []);
        return;
      }
      if (payload.type === "line" && payload.text != null) {
        if (pausedRef.current) return;
        setLines((prev) => [...prev, payload.text!]);
        return;
      }
      if (payload.type === "error") {
        setState("error");
        setError(payload.error || "stream_failed");
      }
    },
  });

  return {
    lines,
    state,
    error,
    paused,
    setPaused: setPausedState,
    clear,
  };
}
