import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useWsConnection } from "../hooks/useWsConnection";

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
  const [connected, setConnected] = useState(false);
  const [session, setSession] = useState<SessionOpenedPayload | null>(null);
  const [sessionRefreshKey, setSessionRefreshKey] = useState(0);

  const handleOpen = useCallback(() => {
    setConnected(true);
  }, []);

  const handleClose = useCallback(() => {
    setConnected(false);
    rejectPending(pending.current);
  }, []);

  const handleError = useCallback(() => {
    setConnected(false);
  }, []);

  const handleDispose = useCallback(() => {
    rejectPending(pending.current);
    setConnected(false);
    setSession(null);
  }, []);

  const handleMessage = useCallback((data: string) => {
    try {
      const msg = JSON.parse(data) as {
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
  }, []);

  const { socketRef, reconnecting } = useWsConnection({
    url: enabled ? wsURL() : null,
    resetKey: sessionRefreshKey,
    onOpen: handleOpen,
    onClose: handleClose,
    onError: handleError,
    onDispose: handleDispose,
    onMessage: handleMessage,
  });

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
    [socketRef],
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
