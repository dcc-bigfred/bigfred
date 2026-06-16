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

export interface DccBusOpenedPayload {
  layoutId: number;
  commandStationId: number;
  speedSteps: number;
  heartbeatSecs: number;
  deadmanSecs: number;
  sessionId: string;
}

export type DataPlaneStatus =
  | "idle"
  | "connecting"
  | "open"
  | "closed"
  | "error";

const RECONNECT_INTERVAL_MS = 2_000;
const CONNECT_TIMEOUT_MS = 1_000;

interface DccBusContextValue {
  status: DataPlaneStatus;
  reconnecting: boolean;
  speedSteps: number | null;
  /** Last measured ping/pong RTT in ms, or null before the first sample. */
  pingLatencyMs: number | null;
  states: Map<number, LocoState>;
  subscribe: (addresses: number[]) => Promise<{ ok: boolean; error?: string }>;
  setSpeed: (
    address: number,
    speed: number,
    forward: boolean,
    emergency?: boolean,
  ) => Promise<{ ok: boolean; error?: string }>;
  setTrainSpeed: (
    trainId: number,
    speed: number,
    forward: boolean,
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
  const [reconnecting, setReconnecting] = useState(false);
  const [speedSteps, setSpeedSteps] = useState<number | null>(null);
  const [pingLatencyMs, setPingLatencyMs] = useState<number | null>(null);
  const [states, setStates] = useState<Map<number, LocoState>>(new Map());
  const [lastError, setLastError] = useState<string | null>(null);

  const sockRef = useRef<WebSocket | null>(null);
  const connectGenRef = useRef(0);
  const hadOpenedRef = useRef(false);
  const reconnectTimerRef = useRef<number | null>(null);
  const pending = useRef<
    Map<string, (ack: { ok: boolean; error?: string }) => void>
  >(new Map());
  const pingSentAtRef = useRef<number | null>(null);
  const lastPingRttMsRef = useRef<number | null>(null);

  useEffect(() => {
    if (!wsUrl) {
      setStatus("idle");
      setReconnecting(false);
      setSpeedSteps(null);
      setPingLatencyMs(null);
      setStates(new Map());
      hadOpenedRef.current = false;
      return;
    }

    let disposed = false;
    const resolved = wsUrl.startsWith("ws://") || wsUrl.startsWith("wss://")
      ? wsUrl
      : resolveURL(wsUrl);

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
      setStatus("connecting");
      setLastError(null);
      setPingLatencyMs(null);
      pingSentAtRef.current = null;
      lastPingRttMsRef.current = null;
      if (hadOpenedRef.current) {
        setReconnecting(true);
      }

      const sock = new WebSocket(resolved);
      sockRef.current = sock;

      const connectTimeout = window.setTimeout(() => {
        if (disposed || gen !== connectGenRef.current) {
          return;
        }
        if (sock.readyState === WebSocket.CONNECTING) {
          sock.close();
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

      sock.onopen = () => {
        window.clearTimeout(connectTimeout);
        if (disposed || gen !== connectGenRef.current) {
          return;
        }
        hadOpenedRef.current = true;
        setReconnecting(false);
        setStatus("open");
        setLastError(null);
      };
      sock.onerror = () => {
        window.clearTimeout(connectTimeout);
        if (disposed || gen !== connectGenRef.current) {
          return;
        }
        setStatus("error");
        setLastError("connection_error");
        console.warn("[dcc-bus] WebSocket error", { wsUrl: resolved });
      };
      sock.onclose = () => {
        window.clearTimeout(connectTimeout);
        if (disposed || gen !== connectGenRef.current) {
          return;
        }
        setStatus("closed");
        if (sockRef.current === sock) {
          sockRef.current = null;
        }
        scheduleReconnect();
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
            if (ack.ok) {
              setLastError(null);
            }
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
          case "pong": {
            const sentAt = pingSentAtRef.current;
            if (sentAt == null) {
              break;
            }
            pingSentAtRef.current = null;
            const rttMs = performance.now() - sentAt;
            lastPingRttMsRef.current = rttMs;
            setPingLatencyMs(rttMs);
            break;
          }
          case "dcc-bus.opened": {
            const opened = msg.payload as DccBusOpenedPayload;
            if (opened.speedSteps > 0) {
              setSpeedSteps(opened.speedSteps);
            }
            break;
          }
        }
      };

      const heartbeat = window.setInterval(() => {
        if (sock.readyState !== WebSocket.OPEN) {
          return;
        }
        const payload =
          lastPingRttMsRef.current != null
            ? { lastPingLatencyMs: lastPingRttMsRef.current }
            : {};
        pingSentAtRef.current = performance.now();
        sock.send(JSON.stringify({ type: "ping", payload }));
      }, heartbeatSecs * 1000);

      activeCleanup = () => {
        window.clearTimeout(connectTimeout);
        window.clearInterval(heartbeat);
        sock.onopen = null;
        sock.onerror = null;
        sock.onclose = null;
        sock.onmessage = null;
        if (sockRef.current === sock) {
          sockRef.current = null;
        }
        if (
          sock.readyState === WebSocket.CONNECTING ||
          sock.readyState === WebSocket.OPEN
        ) {
          sock.close();
        }
      };
    };

    connect();

    return () => {
      disposed = true;
      clearReconnect();
      activeCleanup?.();
      pending.current.clear();
      hadOpenedRef.current = false;
      setReconnecting(false);
      setSpeedSteps(null);
      setPingLatencyMs(null);
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

  const subscribe = useCallback(
    (addresses: number[]) => send("loco.subscribe", { addresses }),
    [send],
  );
  const setSpeed = useCallback(
    (address: number, speed: number, forward: boolean, emergency?: boolean) =>
      send("loco.setSpeed", { address, speed, forward, emergency }),
    [send],
  );
  const setTrainSpeed = useCallback(
    (trainId: number, speed: number, forward: boolean) =>
      send("train.setSpeed", { trainId, speed, forward }),
    [send],
  );
  const setFunction = useCallback(
    (address: number, fn: number, on: boolean) =>
      send("loco.setFunction", { address, function: fn, on }),
    [send],
  );
  const emergencyStop = useCallback(
    (reason?: string) => send("system.estop", { reason: reason ?? "" }),
    [send],
  );

  const value = useMemo<DccBusContextValue>(
    () => ({
      status,
      reconnecting,
      speedSteps,
      pingLatencyMs,
      states,
      subscribe,
      setSpeed,
      setTrainSpeed,
      setFunction,
      emergencyStop,
      lastError,
    }),
    [
      status,
      reconnecting,
      speedSteps,
      pingLatencyMs,
      states,
      subscribe,
      setSpeed,
      setTrainSpeed,
      setFunction,
      emergencyStop,
      lastError,
    ],
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

export function useDccBusOptional(): DccBusContextValue | null {
  return useContext(DccBusContext);
}
