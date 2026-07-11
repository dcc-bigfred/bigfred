import { useEffect, useRef } from "react";

import {
  axisToSpeed,
  GAMEPAD_MAX_SPEED_STEP,
  GAMEPAD_SPEED_WINDOW_MS,
  speedButtonStepSize,
  stabilizeGamepadSpeedStep,
  type GamepadMapping,
} from "./gamepadMapping";

interface GamepadControlHandlers {
  onSpeed: (speed: number) => void;
  onDirectionChange: (forward: boolean) => void;
  onFunctionToggle: (fn: number) => void;
  onStop: () => void;
  onAxisEnabledToggle?: () => void;
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
  onAxisEnabledToggle,
}: UseGamepadControlOptions) {
  const mappingRef = useRef(mapping);
  const handlersRef = useRef({
    onSpeed,
    onDirectionChange,
    onFunctionToggle,
    onStop,
    onAxisEnabledToggle,
  });
  const maxSpeedRef = useRef(maxSpeed);
  const currentSpeedRef = useRef(currentSpeed);
  const forwardRef = useRef(forward);
  const prevButtonsRef = useRef<boolean[]>([]);
  const appliedSpeedRef = useRef<number | null>(null);
  const buttonLatchedSpeedRef = useRef(0);
  const windowEndSpeedRef = useRef(0);
  const windowDeadlineRef = useRef(0);

  mappingRef.current = mapping;
  handlersRef.current = {
    onSpeed,
    onDirectionChange,
    onFunctionToggle,
    onStop,
    onAxisEnabledToggle,
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
      buttonLatchedSpeedRef.current = 0;
      windowDeadlineRef.current = 0;
      return;
    }

    appliedSpeedRef.current = currentSpeedRef.current;
    buttonLatchedSpeedRef.current = currentSpeedRef.current;
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

      const now = performance.now();

      if (active.axisEnabled !== false) {
        const axisValue = gp.axes[active.speedAxis] ?? 0;
        const axisSpeed = axisToSpeed(
          axisValue,
          maxSpeedRef.current,
          active,
        );
        // Button steps latch; neutral axis must not pull speed below that floor.
        windowEndSpeedRef.current = Math.max(
          axisSpeed,
          buttonLatchedSpeedRef.current,
        );
        if (now >= windowDeadlineRef.current) {
          emitStabilizedSpeed(now);
        }
      }

      const prev = prevButtonsRef.current;
      const pressed = (i: number) => Boolean(gp.buttons[i]?.pressed);

      if (
        active.stopButton != null &&
        pressed(active.stopButton) &&
        !prev[active.stopButton]
      ) {
        appliedSpeedRef.current = 0;
        buttonLatchedSpeedRef.current = 0;
        windowEndSpeedRef.current = 0;
        windowDeadlineRef.current = now + GAMEPAD_SPEED_WINDOW_MS;
        handlersRef.current.onStop();
      }

      const stepDelta = speedButtonStepSize(
        maxSpeedRef.current,
        active.speedButtonSteps,
      );

      if (
        active.accelerateButton != null &&
        pressed(active.accelerateButton) &&
        !prev[active.accelerateButton]
      ) {
        const next = Math.min(
          maxSpeedRef.current,
          buttonLatchedSpeedRef.current + stepDelta,
        );
        buttonLatchedSpeedRef.current = next;
        appliedSpeedRef.current = next;
        windowEndSpeedRef.current = next;
        windowDeadlineRef.current = now + GAMEPAD_SPEED_WINDOW_MS;
        handlersRef.current.onSpeed(next);
      }

      if (
        active.decelerateButton != null &&
        pressed(active.decelerateButton) &&
        !prev[active.decelerateButton]
      ) {
        const next = Math.max(0, buttonLatchedSpeedRef.current - stepDelta);
        buttonLatchedSpeedRef.current = next;
        appliedSpeedRef.current = next;
        windowEndSpeedRef.current = next;
        windowDeadlineRef.current = now + GAMEPAD_SPEED_WINDOW_MS;
        handlersRef.current.onSpeed(next);
      }

      if (
        active.reverseButton != null &&
        pressed(active.reverseButton) &&
        !prev[active.reverseButton]
      ) {
        handlersRef.current.onDirectionChange(!forwardRef.current);
      }

      if (
        active.axisToggleButton != null &&
        pressed(active.axisToggleButton) &&
        !prev[active.axisToggleButton]
      ) {
        handlersRef.current.onAxisEnabledToggle?.();
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
      buttonLatchedSpeedRef.current = 0;
      windowDeadlineRef.current = 0;
    };
  }, [disabled, mapping, gamepadIndex]);
}
