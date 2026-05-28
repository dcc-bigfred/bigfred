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
  status: "running" | "starting" | "stopped" | "draining" | "degraded";
  reason?: string;
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
  session: SessionOpenedPayload | null;
  setCommandStation: (csID: number) => Promise<{ ok: boolean; error?: string }>;
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
  const [connected, setConnected] = useState(false);
  const [session, setSession] = useState<SessionOpenedPayload | null>(null);

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

  const sendAction = useCallback<SocketContextValue["sendAction"]>(
    (type, payload) =>
      new Promise((resolve) => {
        const socket = socketRef.current;
        if (!socket || socket.readyState !== WebSocket.OPEN) {
          resolve({ ok: false, error: "control_offline" });
          return;
        }
        const id = nextRequestID();
        pending.current.set(id, resolve);
        socket.send(JSON.stringify({ type, id, payload }));
        // Safety net: drop the resolver after 12 s so a buggy
        // backend doesn't pin memory indefinitely.
        window.setTimeout(() => {
          if (pending.current.delete(id)) {
            resolve({ ok: false, error: "ack_timeout" });
          }
        }, 12_000);
      }),
    [],
  );

  const setCommandStationAckTimeoutMs = 45_000;

  const setCommandStation = useCallback(
    (csID: number) =>
      new Promise<{ ok: boolean; error?: string }>((resolve) => {
        const socket = socketRef.current;
        if (!socket || socket.readyState !== WebSocket.OPEN) {
          resolve({ ok: false, error: "control_offline" });
          return;
        }
        const id = nextRequestID();
        pending.current.set(id, resolve);
        socket.send(
          JSON.stringify({
            type: "session.setCommandStation",
            id,
            payload: { commandStationId: csID },
          }),
        );
        window.setTimeout(() => {
          if (pending.current.delete(id)) {
            resolve({ ok: false, error: "ack_timeout" });
          }
        }, setCommandStationAckTimeoutMs);
      }),
    [],
  );

  useEffect(() => {
    if (!enabled) {
      setConnected(false);
      setSession(null);
      return;
    }

    const socket = new WebSocket(wsURL());
    socketRef.current = socket;

    socket.onopen = () => setConnected(true);
    socket.onclose = () => {
      setConnected(false);
      setSession(null);
      socketRef.current = null;
    };

    socket.onmessage = (ev) => {
      try {
        const msg = JSON.parse(String(ev.data)) as {
          type?: string;
          id?: string;
          payload?: unknown;
        };
        if (!msg.type) return;
        if (msg.type === "ack" && msg.id) {
          const resolver = pending.current.get(msg.id);
          if (resolver) {
            pending.current.delete(msg.id);
            const ack = (msg.payload as { ok?: boolean; error?: string }) ?? {};
            resolver({ ok: Boolean(ack.ok), error: ack.error });
          }
          return;
        }
        if (msg.type === "session.opened") {
          setSession((msg.payload as SessionOpenedPayload) ?? null);
        }
        if (msg.type === "session.commandStationChanged") {
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
              next.availableCommandStations[idx] = {
                ...next.availableCommandStations[idx],
                wsUrl: p.wsUrl,
              };
            }
            if (
              p.commandStationId > 0 &&
              (p.status === "running" || p.status === "starting")
            ) {
              next.currentSession = { commandStationId: p.commandStationId };
            } else if (
              p.status === "stopped" ||
              p.status === "degraded" ||
              p.commandStationId === 0
            ) {
              next.currentSession = undefined;
            }
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
        socket.send(JSON.stringify({ type: "ping" }));
      }
    }, 30_000);

    return () => {
      window.clearInterval(ping);
      socket.close();
      socketRef.current = null;
      setConnected(false);
      setSession(null);
    };
  }, [enabled]);

  const value = useMemo<SocketContextValue>(
    () => ({
      subscribe,
      sendAction,
      connected,
      session,
      setCommandStation,
    }),
    [subscribe, sendAction, connected, session, setCommandStation],
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
