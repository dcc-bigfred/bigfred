import { useCallback, useEffect, useState } from "react";
import type { ConnectedGamepad } from "./useGamepads";
import {
  type GamepadMapping,
  loadGamepadMapping,
  loadLastGamepadMapping,
  saveGamepadMapping,
} from "./gamepadMapping";

/**
 * Persisted gamepad mapping for Throttle.
 *
 * Hydrates eagerly from the last confirmed mapping (even before a pad is
 * visible), then prefers the exact Gamepad.id key once a controller appears.
 * Only confirmed configs should be written via `persist`.
 */
export function usePersistedGamepadMapping(gamepads: ConnectedGamepad[]): {
  mapping: GamepadMapping | null;
  setMapping: React.Dispatch<React.SetStateAction<GamepadMapping | null>>;
  persist: (next: GamepadMapping) => void;
} {
  const [mapping, setMapping] = useState<GamepadMapping | null>(() =>
    loadLastGamepadMapping(),
  );

  useEffect(() => {
    if (gamepads.length === 0) return;
    const padId = gamepads[0].id;
    setMapping((prev) => {
      if (prev?.gamepadId === padId) return prev;
      return loadGamepadMapping(padId);
    });
  }, [gamepads]);

  const persist = useCallback((next: GamepadMapping) => {
    saveGamepadMapping(next);
    setMapping(next);
  }, []);

  return { mapping, setMapping, persist };
}
