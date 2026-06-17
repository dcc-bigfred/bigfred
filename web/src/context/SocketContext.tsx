import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

type EventHandler = (payload: unknown) => void;

export const CommandStationStatus = {
  Running: "running",
  Starting: "starting",
  Stopped: "stopped",
  Draining: "draining",
  Degraded: "degraded",
} as const;

export type CommandStationStatus =
  (typeof CommandStationStatus)[keyof typeof CommandStationStatus];

const WsMessageType = {
  Ack: "ack",
  Ping: "ping",
  SessionOpened: "session.opened",
  SessionCommandStationChanged: "session.commandStationChanged",
  SessionCommandStationCatalogChanged: "session.commandStationCatalogChanged",
  SessionSetCommandStation: "session.setCommandStation",
} as const;

// Available command station entry shipped on `session.opened`. The
// `wsUrl` field is either the reverse-proxy path (e.g.
// "/api/v1/dcc-bus/2/ws") or a fully-qualified ws:// URL when the
// backend runs in direct mode (--dcc-bus-proxy=false). The SPA
// resolves the final URL against `window.location` when the field
// starts with "/".
export interface AvailableCommandStation {
  id: number;
  name: string;
  kind: string;
  speedSteps: number;
  wsUrl: string | null;
}

export interface SessionOpenedPayload {
  sessionId: string;
  layoutId: number;
  availableCommandStations: AvailableCommandStation[];
  currentSession?: {
    commandStationId: number;
  };
}

export interface CommandStationChangedPayload {
  commandStationId: number;
  wsUrl: string | null;
  status: CommandStationStatus;
  reason?: string;
}

export interface CommandStationCatalogChangedPayload {
  commandStationId: number;
  name: string;
  kind: string;
  speedSteps: number;
}

// Pending request → ack resolver. Used by `sendAction` so the caller
// can `await sendAction(...)` and react to ack.ok / ack.error.
type PendingResolver = (ack: { ok: boolean; error?: string }) => void;

const RECONNECT_INTERVAL_MS = 2_000;
const CONNECT_TIMEOUT_MS = 1_000;

interface SocketContextValue {
  subscribe: (eventType: string, handler: EventHandler) => () => void;
  sendAction: (
    type: string,
    payload: unknown,
  ) => Promise<{ ok: boolean; error?: string }>;
  connected: boolean;
  reconnecting: boolean;
  session: SessionOpenedPayload | null;
  setCommandStation: (csID: number) => Promise<{ ok: boolean; error?: string }>;
  refreshSession: () => void;
}

const SocketContext = createContext<SocketContextValue | null>(null);

function wsURL(): string {
  const base = (import.meta.env.VITE_API_BASE ?? "") as string;
  if (base) {
    const url = new URL(base);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    url.pathname = "/api/v1/ws";
    url.search = "";
    return url.toString();
  }
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/api/v1/ws`;
}

function nextRequestID(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function rejectPending(pending: Map<string, PendingResolver>) {
  for (const resolver of pending.values()) {
    resolver({ ok: false, error: "control_offline" });
  }
  pending.clear();
}

// waitForControlSocket resolves once the active control-plane socket
// is OPEN. This avoids a race where React's `connected` flag is still
// true while socketRef was cleared during a reconnect cleanup.
function waitForControlSocket(
  getSocket: () => WebSocket | null,
  timeoutMs: number,
): Promise<WebSocket | null> {
  return new Promise((resolve) => {
    const started = Date.now();
    const tick = () => {
      const socket = getSocket();
      if (socket?.readyState === WebSocket.OPEN) {
        resolve(socket);
        return;
      }
      if (!socket || socket.readyState === WebSocket.CLOSED) {
        resolve(null);
        return;
      }
      if (Date.now() - started >= timeoutMs) {
        resolve(null);
        return;
      }
      window.setTimeout(tick, 50);
    };
    tick();
  });
}

// SocketProvider maintains one WebSocket per authenticated session.
export function SocketProvider({
  enabled,
  children,
}: {
  enabled: boolean;
  children: ReactNode;
}) {
  const handlers = useRef(new Map<string, Set<EventHandler>>());
  const pending = useRef(new Map<string, PendingResolver>());
  const socketRef = useRef<WebSocket | null>(null);
  const connectGenRef = useRef(0);
  const hadOpenedRef = useRef(false);
  const reconnectTimerRef = useRef<number | null>(null);
  const [connected, setConnected] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const [session, setSession] = useState<SessionOpenedPayload | null>(null);
  const [sessionRefreshKey, setSessionRefreshKey] = useState(0);

  const subscribe = useCallback((eventType: string, handler: EventHandler) => {
    let set = handlers.current.get(eventType);
    if (!set) {
      set = new Set();
      handlers.current.set(eventType, set);
    }
    set.add(handler);
    return () => {
      set?.delete(handler);
    };
  }, []);

  const sendOnControlSocket = useCallback(
    (
      type: string,
      payload: unknown,
      ackTimeoutMs: number,
    ): Promise<{ ok: boolean; error?: string }> =>
      waitForControlSocket(() => socketRef.current, 5_000).then((socket) => {
        if (!socket) {
          return { ok: false, error: "control_offline" };
        }
        return new Promise<{ ok: boolean; error?: string }>((resolve) => {
          const id = nextRequestID();
          pending.current.set(id, resolve);
          socket.send(JSON.stringify({ type, id, payload }));
          window.setTimeout(() => {
            if (pending.current.delete(id)) {
              resolve({ ok: false, error: "ack_timeout" });
            }
          }, ackTimeoutMs);
        });
      }),
    [],
  );

  const sendAction = useCallback<SocketContextValue["sendAction"]>(
    (type, payload) => sendOnControlSocket(type, payload, 12_000),
    [sendOnControlSocket],
  );

  const setCommandStationAckTimeoutMs = 45_000;

  const setCommandStation = useCallback(
    (csID: number) =>
      sendOnControlSocket(
        WsMessageType.SessionSetCommandStation,
        { commandStationId: csID },
        setCommandStationAckTimeoutMs,
      ),
    [sendOnControlSocket],
  );

  const refreshSession = useCallback(() => {
    setSessionRefreshKey((k) => k + 1);
  }, []);

  useEffect(() => {
    if (!enabled) {
      setConnected(false);
      setReconnecting(false);
      setSession(null);
      hadOpenedRef.current = false;
      return;
    }

    let disposed = false;
    const url = wsURL();

    const clearReconnect = () => {
      if (reconnectTimerRef.current != null) {
        window.clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };

    let activeCleanup: (() => void) | undefined;

    const connect = () => {
      if (disposed) {
        return;
      }
      clearReconnect();
      activeCleanup?.();

      const gen = ++connectGenRef.current;
      setConnected(false);
      if (hadOpenedRef.current) {
        setReconnecting(true);
      }

      const socket = new WebSocket(url);
      socketRef.current = socket;

      const connectTimeout = window.setTimeout(() => {
        if (disposed || gen !== connectGenRef.current) {
          return;
        }
        if (socket.readyState === WebSocket.CONNECTING) {
          socket.close();
        }
      }, CONNECT_TIMEOUT_MS);

      const scheduleReconnect = () => {
        if (disposed) {
          return;
        }
        if (hadOpenedRef.current) {
          setReconnecting(true);
        }
        reconnectTimerRef.current = window.setTimeout(
          connect,
          RECONNECT_INTERVAL_MS,
        );
      };

      socket.onopen = () => {
        window.clearTimeout(connectTimeout);
        if (disposed || gen !== connectGenRef.current) {
          return;
        }
        hadOpenedRef.current = true;
        setReconnecting(false);
        setConnected(true);
      };
      socket.onerror = () => {
        window.clearTimeout(connectTimeout);
        if (disposed || gen !== connectGenRef.current) {
          return;
        }
        setConnected(false);
      };
      socket.onclose = () => {
        window.clearTimeout(connectTimeout);
        if (disposed || gen !== connectGenRef.current) {
          return;
        }
        setConnected(false);
        rejectPending(pending.current);
        if (socketRef.current === socket) {
          socketRef.current = null;
        }
        scheduleReconnect();
      };

      socket.onmessage = (ev) => {
        try {
          const msg = JSON.parse(String(ev.data)) as {
            type?: string;
            id?: string;
            payload?: unknown;
          };
          if (!msg.type) return;
          if (msg.type === WsMessageType.Ack && msg.id) {
            const resolver = pending.current.get(msg.id);
            if (resolver) {
              pending.current.delete(msg.id);
              const ack =
                (msg.payload as { ok?: boolean; error?: string }) ?? {};
              resolver({ ok: Boolean(ack.ok), error: ack.error });
            }
            return;
          }
          if (msg.type === WsMessageType.SessionOpened) {
            setSession((msg.payload as SessionOpenedPayload) ?? null);
          }
          if (msg.type === WsMessageType.SessionCommandStationChanged) {
            const p = msg.payload as CommandStationChangedPayload;
            setSession((prev) => {
              if (!prev) return prev;
              const next: SessionOpenedPayload = { ...prev };
              const idx = next.availableCommandStations.findIndex(
                (cs) => cs.id === p.commandStationId,
              );
              if (idx >= 0) {
                next.availableCommandStations = [
                  ...next.availableCommandStations,
                ];
                const entry = { ...next.availableCommandStations[idx] };
                // Lazy-spawn sends wsUrl=null while starting. Keep the
                // previous URL so DccBusProvider is not torn down mid-
                // CONNECTING (that surfaces as "closed before established").
                if (p.wsUrl) {
                  entry.wsUrl = p.wsUrl;
                } else if (
                  p.status === CommandStationStatus.Stopped ||
                  p.status === CommandStationStatus.Degraded ||
                  p.commandStationId === 0
                ) {
                  entry.wsUrl = null;
                }
                next.availableCommandStations[idx] = entry;
              }
              if (
                p.commandStationId > 0 &&
                (p.status === CommandStationStatus.Running ||
                  p.status === CommandStationStatus.Starting)
              ) {
                next.currentSession = { commandStationId: p.commandStationId };
              } else if (
                p.status === CommandStationStatus.Stopped ||
                p.status === CommandStationStatus.Degraded ||
                p.commandStationId === 0
              ) {
                next.currentSession = undefined;
              }
              return next;
            });
          }
          if (msg.type === WsMessageType.SessionCommandStationCatalogChanged) {
            const p = msg.payload as CommandStationCatalogChangedPayload;
            setSession((prev) => {
              if (!prev) return prev;
              const idx = prev.availableCommandStations.findIndex(
                (cs) => cs.id === p.commandStationId,
              );
              if (idx < 0) return prev;
              const next: SessionOpenedPayload = { ...prev };
              next.availableCommandStations = [
                ...prev.availableCommandStations,
              ];
              next.availableCommandStations[idx] = {
                ...next.availableCommandStations[idx],
                name: p.name,
                kind: p.kind,
                speedSteps: p.speedSteps,
              };
              return next;
            });
          }
          handlers.current.get(msg.type)?.forEach((fn) => fn(msg.payload));
        } catch {
          // malformed frame — ignore
        }
      };

      const ping = window.setInterval(() => {
        if (socket.readyState === WebSocket.OPEN) {
          socket.send(JSON.stringify({ type: WsMessageType.Ping }));
        }
      }, 30_000);

      activeCleanup = () => {
        window.clearTimeout(connectTimeout);
        window.clearInterval(ping);
        socket.onopen = null;
        socket.onerror = null;
        socket.onclose = null;
        socket.onmessage = null;
        if (socketRef.current === socket) {
          socketRef.current = null;
        }
        if (
          socket.readyState === WebSocket.CONNECTING ||
          socket.readyState === WebSocket.OPEN
        ) {
          socket.close();
        }
      };
    };

    connect();

    return () => {
      disposed = true;
      clearReconnect();
      activeCleanup?.();
      rejectPending(pending.current);
      hadOpenedRef.current = false;
      setConnected(false);
      setReconnecting(false);
      setSession(null);
    };
  }, [enabled, sessionRefreshKey]);

  const value = useMemo<SocketContextValue>(
    () => ({
      subscribe,
      sendAction,
      connected,
      reconnecting,
      session,
      setCommandStation,
      refreshSession,
    }),
    [
      subscribe,
      sendAction,
      connected,
      reconnecting,
      session,
      setCommandStation,
      refreshSession,
    ],
  );

  return (
    <SocketContext.Provider value={value}>{children}</SocketContext.Provider>
  );
}

export function useSocket(): SocketContextValue {
  const ctx = useContext(SocketContext);
  if (!ctx) {
    throw new Error("useSocket must be used within SocketProvider");
  }
  return ctx;
}
