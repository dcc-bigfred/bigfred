import { useEffect, useRef } from "react";

import {
  popThrottleHardwareKeysHandler,
  pushThrottleHardwareKeysHandler,
} from "./throttleHardwareKeysRegistry";
import { applyThrottleHardwareKeyStep } from "./volumeKeyStepSize";

interface Options {
  maxSpeed: number;
  currentSpeed: number;
  onSpeed: (speed: number) => void;
  /** When true, do not register the global handler. */
  disabled?: boolean;
}

/**
 * Registers window.__bigfredThrottleHardwareKeys for the Android shell.
 * Uses a stack so takeover overlays take priority over the main throttle page.
 */
export function useThrottleHardwareKeys({
  maxSpeed,
  currentSpeed,
  onSpeed,
  disabled = false,
}: Options): void {
  const stateRef = useRef({ maxSpeed, currentSpeed, onSpeed });
  stateRef.current = { maxSpeed, currentSpeed, onSpeed };

  useEffect(() => {
    if (disabled) return;

    const handler = (direction: number) => {
      const { maxSpeed: max, currentSpeed: cur, onSpeed: set } =
        stateRef.current;
      set(applyThrottleHardwareKeyStep(direction, cur, max));
    };

    pushThrottleHardwareKeysHandler(handler);
    return () => {
      popThrottleHardwareKeysHandler(handler);
    };
  }, [disabled]);
}
