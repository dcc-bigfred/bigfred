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
  /** When true, direction events are ignored (handler stays registered). */
  disabled?: boolean;
}

/**
 * Registers window.__bigfredThrottleHardwareKeys for the Android shell.
 * Uses a stack so takeover overlays take priority over the main throttle page.
 *
 * Registers once per mount (disabled is checked inside the handler) so remount
 * churn cannot reorder the stack under an active takeover overlay.
 * Tracks the last applied speed locally so Android key-repeat does not lose
 * steps between React re-renders.
 */
export function useThrottleHardwareKeys({
  maxSpeed,
  currentSpeed,
  onSpeed,
  disabled = false,
}: Options): void {
  const stateRef = useRef({ maxSpeed, currentSpeed, onSpeed, disabled });
  stateRef.current = { maxSpeed, currentSpeed, onSpeed, disabled };

  const appliedSpeedRef = useRef(currentSpeed);
  useEffect(() => {
    appliedSpeedRef.current = currentSpeed;
  }, [currentSpeed]);

  useEffect(() => {
    const handler = (direction: number) => {
      const { maxSpeed: max, onSpeed: set, disabled: isDisabled } =
        stateRef.current;
      if (isDisabled) return;
      const next = applyThrottleHardwareKeyStep(
        direction,
        appliedSpeedRef.current,
        max,
      );
      appliedSpeedRef.current = next;
      set(next);
    };

    pushThrottleHardwareKeysHandler(handler);
    return () => {
      popThrottleHardwareKeysHandler(handler);
    };
  }, []);
}
