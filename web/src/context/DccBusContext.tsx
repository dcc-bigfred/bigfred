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

// LocoState is the wire shape dcc-bus pushes on every authoritative
// state change. Mirrors `protocol.LocoStatePayload` in Go.
export interface LocoState {
  address: number;
  speed: number;
  forward: boolean;
  functions: boolean[];
  controlledByUserId?: number;
  source?: string;
  at: number;
}

export type DataPlaneStatus =
  | "idle"
  | "connecting"
  | "open"
  | "closed"
  | "error";

interface DccBusContextValue {
  status: DataPlaneStatus;
  states: Map<number, LocoState>;
  subscribe: (addresses: number[]) => Promise<{ ok: boolean; error?: string }>;
  setSpeed: (
    address: number,
    speed: number,
    forward: boolean,
    emergency?: boolean,
  ) => Promise<{ ok: boolean; error?: string }>;
  setFunction: (
    address: number,
    fn: number,
    on: boolean,
  ) => Promise<{ ok: boolean; error?: string }>;
  emergencyStop: (reason?: string) => Promise<{ ok: boolean; error?: string }>;
  lastError: string | null;
}

const DccBusContext = createContext<DccBusContextValue | null>(null);

function resolveURL(wsUrl: string): string {
  if (wsUrl.startsWith("ws://") || wsUrl.startsWith("wss://")) {
    return wsUrl;
  }
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}${wsUrl}`;
}

function newID(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

// DccBusProvider opens / closes a WebSocket whenever the `wsUrl`
// prop changes. Multiple commands flow through the same connection
// — the provider owns the pending-ack table so callers can `await`
// the daemon's response.
export function DccBusProvider({
  wsUrl,
  heartbeatSecs = 5,
  children,
}: {
  wsUrl: string | null;
  heartbeatSecs?: number;
  children: ReactNode;
}) {
  const [status, setStatus] = useState<DataPlaneStatus>("idle");
  const [states, setStates] = useState<Map<number, LocoState>>(new Map());
  const [lastError, setLastError] = useState<string | null>(null);

  const sockRef = useRef<WebSocket | null>(null);
  const pending = useRef<
    Map<string, (ack: { ok: boolean; error?: string }) => void>
  >(new Map());

  useEffect(() => {
    if (!wsUrl) {
      setStatus("idle");
      setStates(new Map());
      return;
    }
    setStatus("connecting");
    const sock = new WebSocket(resolveURL(wsUrl));
    sockRef.current = sock;

    sock.onopen = () => setStatus("open");
    sock.onerror = () => {
      setStatus("error");
      setLastError("connection_error");
      console.warn("[dcc-bus] WebSocket error", { wsUrl });
    };
    sock.onclose = () => {
      setStatus("closed");
      sockRef.current = null;
    };
    sock.onmessage = (ev) => {
      let msg: { type?: string; id?: string; payload?: unknown };
      try {
        msg = JSON.parse(String(ev.data));
      } catch {
        return;
      }
      switch (msg.type) {
        case "ack": {
          if (!msg.id) return;
          const resolver = pending.current.get(msg.id);
          if (!resolver) return;
          pending.current.delete(msg.id);
          const ack =
            (msg.payload as { ok?: boolean; error?: string }) ?? { ok: false };
          resolver({ ok: Boolean(ack.ok), error: ack.error });
          break;
        }
        case "loco.state": {
          const state = msg.payload as LocoState;
          setStates((prev) => {
            const next = new Map(prev);
            next.set(state.address, state);
            return next;
          });
          break;
        }
        case "loco.error": {
          const err = msg.payload as {
            address?: number;
            code?: string;
            detail?: string;
          };
          if (err?.code) {
            setLastError(err.code);
            console.warn("[dcc-bus] loco.error", err);
          }
          break;
        }
        case "dcc-bus.opened": {
          // Welcome frame; nothing to do beyond keeping status open.
          break;
        }
      }
    };

    const heartbeat = window.setInterval(() => {
      if (sock.readyState === WebSocket.OPEN) {
        sock.send(JSON.stringify({ type: "ping" }));
      }
    }, heartbeatSecs * 1000);

    return () => {
      window.clearInterval(heartbeat);
      sock.close();
      sockRef.current = null;
      pending.current.clear();
      setStates(new Map());
      setStatus("idle");
    };
  }, [wsUrl, heartbeatSecs]);

  const send = useCallback(
    (type: string, payload: unknown) =>
      new Promise<{ ok: boolean; error?: string }>((resolve) => {
        const sock = sockRef.current;
        if (!sock || sock.readyState !== WebSocket.OPEN) {
          resolve({ ok: false, error: "dcc_bus_offline" });
          return;
        }
        const id = newID();
        pending.current.set(id, resolve);
        sock.send(JSON.stringify({ type, id, payload }));
        window.setTimeout(() => {
          if (pending.current.delete(id)) {
            resolve({ ok: false, error: "ack_timeout" });
          }
        }, 8_000);
      }),
    [],
  );

  const value = useMemo<DccBusContextValue>(
    () => ({
      status,
      states,
      subscribe: (addresses) => send("loco.subscribe", { addresses }),
      setSpeed: (address, speed, forward, emergency) =>
        send("loco.setSpeed", { address, speed, forward, emergency }),
      setFunction: (address, fn, on) =>
        send("loco.setFunction", { address, function: fn, on }),
      emergencyStop: (reason) => send("system.estop", { reason: reason ?? "" }),
      lastError,
    }),
    [status, states, send, lastError],
  );

  return (
    <DccBusContext.Provider value={value}>{children}</DccBusContext.Provider>
  );
}

export function useDccBus(): DccBusContextValue {
  const ctx = useContext(DccBusContext);
  if (!ctx) {
    throw new Error("useDccBus must be used within DccBusProvider");
  }
  return ctx;
}
