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
  const [speedSteps, setSpeedSteps] = useState<number | null>(null);
  const [pingLatencyMs, setPingLatencyMs] = useState<number | null>(null);
  const [states, setStates] = useState<Map<number, LocoState>>(new Map());
  const [lastError, setLastError] = useState<string | null>(null);

  const pending = useRef<
    Map<string, (ack: { ok: boolean; error?: string }) => void>
  >(new Map());
  const pingSentAtRef = useRef<number | null>(null);
  const lastPingRttMsRef = useRef<number | null>(null);

  const resolvedUrl = wsUrl != null ? resolveURL(wsUrl) : null;

  const handleConnecting = useCallback(() => {
    pingSentAtRef.current = null;
    lastPingRttMsRef.current = null;
    setPingLatencyMs(null);
    setLastError(null);
    setStatus("connecting");
  }, []);

  const handleOpen = useCallback(() => {
    setStatus("open");
  }, []);

  const handleClose = useCallback(() => {
    setStatus("closed");
    // Fail in-flight commands immediately instead of waiting out the
    // ack timeout while the socket is already gone.
    for (const resolver of pending.current.values()) {
      resolver({ ok: false, error: "dcc_bus_offline" });
    }
    pending.current.clear();
  }, []);

  const handleDispose = useCallback(() => {
    for (const resolver of pending.current.values()) {
      resolver({ ok: false, error: "dcc_bus_offline" });
    }
    pending.current.clear();
    setStatus("idle");
    setSpeedSteps(null);
    setPingLatencyMs(null);
    setStates(new Map());
  }, []);

  const handleError = useCallback(() => {
    setStatus("error");
    setLastError("connection_error");
    console.warn("[dcc-bus] WebSocket error", { wsUrl });
  }, [wsUrl]);

  const handlePong = useCallback(() => {
    const sentAt = pingSentAtRef.current;
    if (sentAt == null) return;
    pingSentAtRef.current = null;
    const rttMs = performance.now() - sentAt;
    lastPingRttMsRef.current = rttMs;
    setPingLatencyMs(rttMs);
  }, []);

  const buildPingFrame = useCallback(() => {
    const payload =
      lastPingRttMsRef.current != null
        ? { lastPingLatencyMs: lastPingRttMsRef.current }
        : {};
    pingSentAtRef.current = performance.now();
    return JSON.stringify({ type: "ping", payload });
  }, []);

  const handleMessage = useCallback((data: string) => {
    let msg: { type?: string; id?: string; payload?: unknown };
    try {
      msg = JSON.parse(data);
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
      // "pong" is consumed by useWsConnection (watchdog + onPong callback)
      case "dcc-bus.opened": {
        const opened = msg.payload as DccBusOpenedPayload;
        if (opened.speedSteps > 0) {
          setSpeedSteps(opened.speedSteps);
        }
        break;
      }
    }
  }, []);

  const { socketRef, reconnecting } = useWsConnection({
    url: resolvedUrl,
    pingIntervalMs: heartbeatSecs * 1_000,
    buildPingFrame,
    onConnecting: handleConnecting,
    onOpen: handleOpen,
    onClose: handleClose,
    onDispose: handleDispose,
    onError: handleError,
    onMessage: handleMessage,
    onPong: handlePong,
  });

  const send = useCallback(
    (type: string, payload: unknown) =>
      new Promise<{ ok: boolean; error?: string }>((resolve) => {
        const sock = socketRef.current;
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
    [socketRef],
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
