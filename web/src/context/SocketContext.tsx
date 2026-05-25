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

interface SocketContextValue {
  subscribe: (eventType: string, handler: EventHandler) => () => void;
  connected: boolean;
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

// SocketProvider maintains one WebSocket per authenticated session.
export function SocketProvider({
  enabled,
  children,
}: {
  enabled: boolean;
  children: ReactNode;
}) {
  const handlers = useRef(new Map<string, Set<EventHandler>>());
  const [connected, setConnected] = useState(false);

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

  useEffect(() => {
    if (!enabled) {
      setConnected(false);
      return;
    }

    const socket = new WebSocket(wsURL());

    socket.onopen = () => setConnected(true);
    socket.onclose = () => setConnected(false);

    socket.onmessage = (ev) => {
      try {
        const msg = JSON.parse(String(ev.data)) as {
          type?: string;
          payload?: unknown;
        };
        if (!msg.type) return;
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
      setConnected(false);
    };
  }, [enabled]);

  const value = useMemo<SocketContextValue>(
    () => ({ subscribe, connected }),
    [subscribe, connected],
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
