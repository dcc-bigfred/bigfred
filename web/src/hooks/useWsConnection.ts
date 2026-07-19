import { useEffect, useRef, useState, type MutableRefObject } from "react";

import { isSessionUnauthorized } from "../api/auth";
import { notifySessionExpired } from "../auth/sessionExpiry";

export interface UseWsConnectionOptions {
  /** WebSocket URL to connect to. Pass `null` to stay disconnected. */
  url: string | null;
  /** Max time to wait while CONNECTING before aborting. Default 2 000 ms. */
  connectTimeoutMs?: number;
  /** Delay before the first retry after a disconnect. Default 250 ms. */
  reconnectIntervalMs?: number;
  /** Added to the reconnect delay on each consecutive failed attempt (linear backoff). Default 50 ms. */
  reconnectBackoffStepMs?: number;
  /** Upper bound for the reconnect delay. Default 2 000 ms. */
  reconnectMaxMs?: number;
  /** Interval between outgoing ping frames. Default 10 000 ms. */
  pingIntervalMs?: number;
  /**
   * Maximum time without a pong before the socket is treated as stale and
   * forcibly closed (triggering a reconnect). Default 15 000 ms.
   */
  pongTimeoutMs?: number;
  /**
   * Incrementing this value forces a fresh connect cycle even when the URL
   * has not changed — useful for session-refresh scenarios.
   */
  resetKey?: number;
  /**
   * Returns the serialised ping frame to send each heartbeat tick. Called
   * every tick so dynamic payloads (e.g. last measured RTT) are always fresh.
   * Defaults to `{"type":"ping"}`.
   */
  buildPingFrame?: () => string;
  /** Called at the very start of each connection attempt. */
  onConnecting?: () => void;
  /** Called once the WebSocket is OPEN and ready. */
  onOpen?: () => void;
  /**
   * Called when the socket closes naturally — a reconnect will follow.
   * Use this to reject in-flight requests and update application state.
   */
  onClose?: () => void;
  /**
   * Called during effect cleanup (URL became `null` or component unmounted).
   * No reconnect follows. Use this to reset application state to idle.
   */
  onDispose?: () => void;
  /** Called on a WebSocket error event. */
  onError?: () => void;
  /**
   * Called when the handshake fails because the session is no longer
   * valid (HTTP 401). Reconnect is suppressed after this fires.
   */
  onUnauthorized?: () => void;
  /** Called for every incoming message that is not a `pong` frame. */
  onMessage?: (data: string) => void;
  /** Called when a pong frame is received (after the internal watchdog timestamp is updated). */
  onPong?: () => void;
}

export interface UseWsConnectionResult {
  /** Ref to the active WebSocket. `current` is OPEN only while connected. */
  socketRef: React.MutableRefObject<WebSocket | null>;
  /** True while reconnecting after having been connected at least once. */
  reconnecting: boolean;
}

const DEFAULT_PING_FRAME = JSON.stringify({ type: "ping" });

async function handleUnauthorizedHandshake(
  openedThisAttempt: boolean,
  disposed: boolean,
  gen: number,
  connectGenRef: MutableRefObject<number>,
  onUnauthorizedRef: MutableRefObject<(() => void) | undefined>,
): Promise<boolean> {
  if (openedThisAttempt) return false;
  if (disposed || gen !== connectGenRef.current) return false;
  if (!(await isSessionUnauthorized())) return false;
  if (disposed || gen !== connectGenRef.current) return false;
  (onUnauthorizedRef.current ?? notifySessionExpired)();
  return true;
}

/** Detach handlers and clear the ref without calling close(). */
function orphanSocket(
  socket: WebSocket,
  socketRef: React.MutableRefObject<WebSocket | null>,
) {
  socket.onopen = null;
  socket.onerror = null;
  socket.onclose = null;
  socket.onmessage = null;
  if (socketRef.current === socket) socketRef.current = null;
}

/**
 * useWsConnection manages a single WebSocket connection with automatic
 * reconnect, linear backoff, configurable connect timeout and a pong
 * watchdog that detects half-open (silent) connections.
 *
 * All callback options (onOpen, onClose, …) are read through refs so they
 * are always up-to-date without being listed as effect dependencies — the
 * connection is only restarted when a structural option (url, timing) changes.
 */
export function useWsConnection({
  url,
  connectTimeoutMs = 2_000,
  reconnectIntervalMs = 250,
  reconnectBackoffStepMs = 50,
  reconnectMaxMs = 2_000,
  pingIntervalMs = 10_000,
  pongTimeoutMs = 15_000,
  resetKey = 0,
  buildPingFrame,
  onConnecting,
  onOpen,
  onClose,
  onDispose,
  onError,
  onUnauthorized,
  onMessage,
  onPong,
}: UseWsConnectionOptions): UseWsConnectionResult {
  const socketRef = useRef<WebSocket | null>(null);
  const connectGenRef = useRef(0);
  const hadOpenedRef = useRef(false);
  const reconnectTimerRef = useRef<number | null>(null);
  const [reconnecting, setReconnecting] = useState(false);

  // Latest-ref pattern: callbacks are always current inside the effect
  // closure without being listed as dependencies (which would restart the
  // connection on every re-render that creates new function identities).
  const onConnectingRef = useRef(onConnecting);
  const onOpenRef = useRef(onOpen);
  const onCloseRef = useRef(onClose);
  const onDisposeRef = useRef(onDispose);
  const onErrorRef = useRef(onError);
  const onUnauthorizedRef = useRef(onUnauthorized);
  const onMessageRef = useRef(onMessage);
  const onPongRef = useRef(onPong);
  const buildPingFrameRef = useRef(buildPingFrame);
  onConnectingRef.current = onConnecting;
  onOpenRef.current = onOpen;
  onCloseRef.current = onClose;
  onDisposeRef.current = onDispose;
  onErrorRef.current = onError;
  onUnauthorizedRef.current = onUnauthorized;
  onMessageRef.current = onMessage;
  onPongRef.current = onPong;
  buildPingFrameRef.current = buildPingFrame;

  useEffect(() => {
    if (!url) {
      hadOpenedRef.current = false;
      setReconnecting(false);
      return;
    }

    let disposed = false;
    // Fast first retry for brief WiFi blips, then linear backoff to a cap so
    // a long outage (server restart) doesn't hammer the radio or drain battery.
    let reconnectDelay = reconnectIntervalMs;

    const clearReconnect = () => {
      if (reconnectTimerRef.current != null) {
        window.clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };

    let activeCleanup: (() => void) | undefined;

    const connect = () => {
      if (disposed) return;
      clearReconnect();
      activeCleanup?.();

      const gen = ++connectGenRef.current;
      if (hadOpenedRef.current) setReconnecting(true);
      onConnectingRef.current?.();

      const socket = new WebSocket(url);
      socketRef.current = socket;
      let lastPongAt = Date.now();
      let openedThisAttempt = false;

      const scheduleReconnect = () => {
        if (disposed) return;
        if (hadOpenedRef.current) setReconnecting(true);
        const delay = reconnectDelay;
        reconnectDelay = Math.min(
          reconnectDelay + reconnectBackoffStepMs,
          reconnectMaxMs,
        );
        reconnectTimerRef.current = window.setTimeout(connect, delay);
      };

      const abortConnecting = () => {
        if (socket.readyState !== WebSocket.CONNECTING) return;
        window.clearTimeout(connectTimeout);
        orphanSocket(socket, socketRef);
        void (async () => {
          if (
            await handleUnauthorizedHandshake(
              openedThisAttempt,
              disposed,
              gen,
              connectGenRef,
              onUnauthorizedRef,
            )
          ) {
            return;
          }
          if (disposed || gen !== connectGenRef.current) return;
          onCloseRef.current?.();
          scheduleReconnect();
        })();
      };

      const connectTimeout = window.setTimeout(() => {
        if (disposed || gen !== connectGenRef.current) return;
        abortConnecting();
      }, connectTimeoutMs);

      socket.onopen = () => {
        window.clearTimeout(connectTimeout);
        if (disposed || gen !== connectGenRef.current) return;
        openedThisAttempt = true;
        hadOpenedRef.current = true;
        reconnectDelay = reconnectIntervalMs;
        lastPongAt = Date.now();
        setReconnecting(false);
        onOpenRef.current?.();
      };

      socket.onerror = () => {
        window.clearTimeout(connectTimeout);
        if (disposed || gen !== connectGenRef.current) return;
        onErrorRef.current?.();
      };

      socket.onclose = () => {
        window.clearTimeout(connectTimeout);
        if (disposed || gen !== connectGenRef.current) return;
        if (socketRef.current === socket) socketRef.current = null;
        void (async () => {
          if (
            await handleUnauthorizedHandshake(
              openedThisAttempt,
              disposed,
              gen,
              connectGenRef,
              onUnauthorizedRef,
            )
          ) {
            return;
          }
          if (disposed || gen !== connectGenRef.current) return;
          onCloseRef.current?.();
          scheduleReconnect();
        })();
      };

      socket.onmessage = (ev) => {
        const data = String(ev.data);
        try {
          const msg = JSON.parse(data) as { type?: string };
          if (msg.type === "pong") {
            lastPongAt = Date.now();
            onPongRef.current?.();
            return;
          }
        } catch {
          // malformed frame — forward to consumer
        }
        onMessageRef.current?.(data);
      };

      const ping = window.setInterval(() => {
        if (socket.readyState !== WebSocket.OPEN) return;
        socket.send(buildPingFrameRef.current?.() ?? DEFAULT_PING_FRAME);
      }, pingIntervalMs);

      const pongWatchdog = window.setInterval(() => {
        if (disposed || gen !== connectGenRef.current) return;
        if (socket.readyState !== WebSocket.OPEN) return;
        if (Date.now() - lastPongAt >= pongTimeoutMs) socket.close();
      }, 1_000);

      activeCleanup = () => {
        window.clearTimeout(connectTimeout);
        window.clearInterval(ping);
        window.clearInterval(pongWatchdog);
        orphanSocket(socket, socketRef);
        if (socket.readyState === WebSocket.OPEN) {
          socket.close();
        }
        // CONNECTING sockets are orphaned, not closed, so a superseded
        // attempt does not trigger the browser warning "WebSocket is
        // closed before the connection is established".
      };
    };

    // Android suspends background tabs and powers down WiFi, which kills
    // the socket and freezes the reconnect timer. Force an immediate retry
    // when the tab returns to the foreground or the network comes back.
    // Also send an immediate keepalive ping when visibility/focus returns so
    // a transient hidden state (WebView overlay) cannot cross the dcc-bus
    // dead-man window (~6s) before the next setInterval tick.
    const sendKeepalivePing = () => {
      const socket = socketRef.current;
      if (!socket || socket.readyState !== WebSocket.OPEN) return;
      socket.send(buildPingFrameRef.current?.() ?? DEFAULT_PING_FRAME);
    };
    const reconnectNow = () => {
      if (disposed) return;
      const socket = socketRef.current;
      if (
        socket &&
        (socket.readyState === WebSocket.OPEN ||
          socket.readyState === WebSocket.CONNECTING)
      ) {
        return;
      }
      connect();
    };
    const onVisible = () => {
      if (document.visibilityState !== "visible") return;
      reconnectNow();
      sendKeepalivePing();
    };
    const onFocus = () => {
      sendKeepalivePing();
    };
    window.addEventListener("online", reconnectNow);
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("focus", onFocus);

    connect();

    return () => {
      disposed = true;
      window.removeEventListener("online", reconnectNow);
      document.removeEventListener("visibilitychange", onVisible);
      window.removeEventListener("focus", onFocus);
      clearReconnect();
      activeCleanup?.();
      // Notify the consumer even though socket handlers are already nulled
      // out, so it can reject in-flight requests and reset state to idle.
      onDisposeRef.current?.();
      hadOpenedRef.current = false;
      setReconnecting(false);
    };
  }, [
    url,
    resetKey,
    connectTimeoutMs,
    reconnectIntervalMs,
    reconnectBackoffStepMs,
    reconnectMaxMs,
    pingIntervalMs,
    pongTimeoutMs,
  ]);

  return { socketRef, reconnecting };
}
