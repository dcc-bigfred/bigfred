import { useCallback, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";

import { getHelpEntry, type HelpEntry } from "../components/help/helpRegistry";

const POSITION_KEY = "bigfred.help.position";
const DISABLED_ROUTES_KEY = "bigfred.help.disabledRoutes";
const DISABLED_GLOBAL_KEY = "bigfred.help.disabledGlobal";

export type HelpPosition = { x: number; y: number };

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

export function useHelpVisibility() {
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

  const visible =
    !!entry && !disabledGlobal && !disabledRoutes.includes(pathname);

  return {
    entry: entry as HelpEntry | null,
    pathname,
    visible,
    position,
    setPosition,
    disableRoute,
    disableGlobal,
  };
}
