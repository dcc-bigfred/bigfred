import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useLocation } from "react-router-dom";

import { getHelpEntry, type HelpEntry } from "../components/help/helpRegistry";

const POSITION_KEY = "bigfred.help.position";
const DISABLED_ROUTES_KEY = "bigfred.help.disabledRoutes";
const DISABLED_GLOBAL_KEY = "bigfred.help.disabledGlobal";

export type HelpPosition = { x: number; y: number };

type HelpVisibilityValue = {
  entry: HelpEntry | null;
  pathname: string;
  visible: boolean;
  position: HelpPosition;
  setPosition: (next: HelpPosition) => void;
  disableRoute: (route: string) => void;
  disableGlobal: () => void;
  /** Clear global + current-route hide flags (localStorage) and show the FAB again. */
  enableHelp: () => void;
  /** Increments when the account menu asks to open the help dialog. */
  openRequestId: number;
  requestOpenHelp: () => void;
  routeDisabled: boolean;
  globalDisabled: boolean;
};

const HelpVisibilityContext = createContext<HelpVisibilityValue | null>(null);

function readJSON<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return fallback;
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

function defaultPosition(): HelpPosition {
  if (typeof window === "undefined") return { x: 24, y: 24 };
  return {
    x: Math.max(16, window.innerWidth - 72),
    y: Math.max(16, window.innerHeight - 72),
  };
}

export function HelpVisibilityProvider({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const entry = useMemo(() => getHelpEntry(pathname), [pathname]);

  const [position, setPositionState] = useState<HelpPosition>(() =>
    readJSON<HelpPosition>(POSITION_KEY, defaultPosition()),
  );
  const [disabledRoutes, setDisabledRoutes] = useState<string[]>(() =>
    readJSON<string[]>(DISABLED_ROUTES_KEY, []),
  );
  const [disabledGlobal, setDisabledGlobal] = useState<boolean>(() =>
    readJSON<boolean>(DISABLED_GLOBAL_KEY, false),
  );
  const [openRequestId, setOpenRequestId] = useState(0);

  const setPosition = useCallback((next: HelpPosition) => {
    setPositionState(next);
    try {
      localStorage.setItem(POSITION_KEY, JSON.stringify(next));
    } catch {
      /* ignore quota */
    }
  }, []);

  const disableRoute = useCallback((route: string) => {
    setDisabledRoutes((prev) => {
      if (prev.includes(route)) return prev;
      const next = [...prev, route];
      try {
        localStorage.setItem(DISABLED_ROUTES_KEY, JSON.stringify(next));
      } catch {
        /* ignore */
      }
      return next;
    });
  }, []);

  const disableGlobal = useCallback(() => {
    setDisabledGlobal(true);
    try {
      localStorage.setItem(DISABLED_GLOBAL_KEY, JSON.stringify(true));
    } catch {
      /* ignore */
    }
  }, []);

  const enableHelp = useCallback(() => {
    setDisabledGlobal(false);
    try {
      localStorage.setItem(DISABLED_GLOBAL_KEY, JSON.stringify(false));
    } catch {
      /* ignore */
    }
    setDisabledRoutes((prev) => {
      if (!prev.includes(pathname)) return prev;
      const next = prev.filter((r) => r !== pathname);
      try {
        localStorage.setItem(DISABLED_ROUTES_KEY, JSON.stringify(next));
      } catch {
        /* ignore */
      }
      return next;
    });
  }, [pathname]);

  const requestOpenHelp = useCallback(() => {
    enableHelp();
    setOpenRequestId((n) => n + 1);
  }, [enableHelp]);

  const routeDisabled = disabledRoutes.includes(pathname);
  const visible = !!entry && !disabledGlobal && !routeDisabled;

  const value = useMemo<HelpVisibilityValue>(
    () => ({
      entry,
      pathname,
      visible,
      position,
      setPosition,
      disableRoute,
      disableGlobal,
      enableHelp,
      openRequestId,
      requestOpenHelp,
      routeDisabled,
      globalDisabled: disabledGlobal,
    }),
    [
      entry,
      pathname,
      visible,
      position,
      setPosition,
      disableRoute,
      disableGlobal,
      enableHelp,
      openRequestId,
      requestOpenHelp,
      routeDisabled,
      disabledGlobal,
    ],
  );

  return (
    <HelpVisibilityContext.Provider value={value}>
      {children}
    </HelpVisibilityContext.Provider>
  );
}

export function useHelpVisibility(): HelpVisibilityValue {
  const ctx = useContext(HelpVisibilityContext);
  if (!ctx) {
    throw new Error("useHelpVisibility must be used within HelpVisibilityProvider");
  }
  return ctx;
}
