import { useEffect, useRef } from "react";

import {
  axisToSpeed,
  GAMEPAD_MAX_SPEED_STEP,
  GAMEPAD_SPEED_WINDOW_MS,
  stabilizeGamepadSpeedStep,
  type GamepadMapping,
} from "./gamepadMapping";

interface GamepadControlHandlers {
  onSpeed: (speed: number) => void;
  onDirectionChange: (forward: boolean) => void;
  onFunctionToggle: (fn: number) => void;
  onStop: () => void;
}

interface UseGamepadControlOptions extends GamepadControlHandlers {
  mapping: GamepadMapping | null;
  gamepadIndex: number | null;
  maxSpeed: number;
  currentSpeed: number;
  forward: boolean;
  disabled?: boolean;
}

/** Polls the gamepad each frame and forwards input to throttle handlers. */
export function useGamepadControl({
  mapping,
  gamepadIndex,
  maxSpeed,
  currentSpeed,
  forward,
  disabled = false,
  onSpeed,
  onDirectionChange,
  onFunctionToggle,
  onStop,
}: UseGamepadControlOptions) {
  const mappingRef = useRef(mapping);
  const handlersRef = useRef({
    onSpeed,
    onDirectionChange,
    onFunctionToggle,
    onStop,
  });
  const maxSpeedRef = useRef(maxSpeed);
  const currentSpeedRef = useRef(currentSpeed);
  const forwardRef = useRef(forward);
  const prevButtonsRef = useRef<boolean[]>([]);
  const appliedSpeedRef = useRef<number | null>(null);
  const windowEndSpeedRef = useRef(0);
  const windowDeadlineRef = useRef(0);

  mappingRef.current = mapping;
  handlersRef.current = {
    onSpeed,
    onDirectionChange,
    onFunctionToggle,
    onStop,
  };
  maxSpeedRef.current = maxSpeed;
  currentSpeedRef.current = currentSpeed;
  forwardRef.current = forward;

  useEffect(() => {
    if (
      disabled ||
      mapping == null ||
      !mapping.enabled ||
      gamepadIndex == null
    ) {
      prevButtonsRef.current = [];
      appliedSpeedRef.current = null;
      windowDeadlineRef.current = 0;
      return;
    }

    appliedSpeedRef.current = currentSpeedRef.current;
    windowDeadlineRef.current = performance.now() + GAMEPAD_SPEED_WINDOW_MS;

    let frame = 0;

    const emitStabilizedSpeed = (now: number) => {
      const target = windowEndSpeedRef.current;
      const base = appliedSpeedRef.current ?? target;
      const next = stabilizeGamepadSpeedStep(
        base,
        target,
        GAMEPAD_MAX_SPEED_STEP,
      );
      if (next !== base) {
        appliedSpeedRef.current = next;
        handlersRef.current.onSpeed(next);
      }
      windowDeadlineRef.current = now + GAMEPAD_SPEED_WINDOW_MS;
    };

    const tick = () => {
      const active = mappingRef.current;
      if (!active?.enabled) {
        frame = requestAnimationFrame(tick);
        return;
      }

      const gp = navigator.getGamepads?.()[gamepadIndex];
      if (!gp?.connected) {
        frame = requestAnimationFrame(tick);
        return;
      }

      const axisValue = gp.axes[active.speedAxis] ?? 0;
      windowEndSpeedRef.current = axisToSpeed(
        axisValue,
        maxSpeedRef.current,
        active,
      );

      const now = performance.now();
      if (now >= windowDeadlineRef.current) {
        emitStabilizedSpeed(now);
      }

      const prev = prevButtonsRef.current;
      const pressed = (i: number) => Boolean(gp.buttons[i]?.pressed);

      if (
        active.stopButton != null &&
        pressed(active.stopButton) &&
        !prev[active.stopButton]
      ) {
        appliedSpeedRef.current = 0;
        windowEndSpeedRef.current = 0;
        windowDeadlineRef.current = now + GAMEPAD_SPEED_WINDOW_MS;
        handlersRef.current.onStop();
      }

      if (
        active.reverseButton != null &&
        pressed(active.reverseButton) &&
        !prev[active.reverseButton]
      ) {
        handlersRef.current.onDirectionChange(!forwardRef.current);
      }

      for (const [fnStr, btnIndex] of Object.entries(active.fnButtons)) {
        const fn = Number(fnStr);
        if (!Number.isFinite(fn)) continue;
        if (pressed(btnIndex) && !prev[btnIndex]) {
          handlersRef.current.onFunctionToggle(fn);
        }
      }

      prevButtonsRef.current = gp.buttons.map((b) => b.pressed);
      frame = requestAnimationFrame(tick);
    };

    frame = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(frame);
      prevButtonsRef.current = [];
      appliedSpeedRef.current = null;
      windowDeadlineRef.current = 0;
    };
  }, [disabled, mapping, gamepadIndex]);
}
