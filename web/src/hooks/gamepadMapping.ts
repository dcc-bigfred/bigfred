const STORAGE_KEY_PREFIX = "bigfred.throttle.gamepad";

/** Default axis index for speed (left stick Y on standard gamepad layout). */
export const DEFAULT_SPEED_AXIS = 1;

export interface GamepadMapping {
  gamepadId: string;
  speedAxis: number;
  invertAxis: boolean;
  /** When false, speed axis and sensitivity are ignored; buttons still work. */
  axisEnabled?: boolean;
  /** Resting axis range learned while idle — values inside mean speed 0. */
  idleAxisMin?: number;
  idleAxisMax?: number;
  reverseButton?: number;
  stopButton?: number;
  accelerateButton?: number;
  decelerateButton?: number;
  /** Discrete speed steps per accelerate/decelerate press (default 20). */
  speedButtonSteps?: number;
  /** DCC function number -> gamepad button index. */
  fnButtons: Record<number, number>;
  /** 0..4 — each step scales axis-to-speed output via GAMEPAD_SPEED_SENSITIVITY_DIVISORS. */
  speedSensitivity?: GamepadSpeedSensitivity;
  enabled: boolean;
}

export type GamepadSpeedSensitivity = 0 | 1 | 2 | 3 | 4;

export const GAMEPAD_SPEED_SENSITIVITY_MAX = 4;

export const GAMEPAD_SPEED_SENSITIVITY_DIVISORS: readonly [
  1.25, 1.5, 1.75, 2, 3,
] = [1.25, 1.5, 1.75, 2, 3];


export const GAMEPAD_IDLE_LEARN_SECONDS = 10;
/** Expand detected idle range by this fraction on each side for a safety margin. */
export const GAMEPAD_IDLE_MAX_BUFFER_RATIO = 0.15;

export const DEFAULT_SPEED_BUTTON_STEPS = 20;
export const MIN_SPEED_BUTTON_STEPS = 2;
export const MAX_SPEED_BUTTON_STEPS = 20;

export function defaultGamepadMapping(gamepadId: string): GamepadMapping {
  return {
    gamepadId,
    speedAxis: DEFAULT_SPEED_AXIS,
    invertAxis: true,
    axisEnabled: true,
    fnButtons: {},
    speedSensitivity: 0,
    speedButtonSteps: DEFAULT_SPEED_BUTTON_STEPS,
    enabled: false,
  };
}

function storageKey(gamepadId: string): string {
  return `${STORAGE_KEY_PREFIX}.${encodeURIComponent(gamepadId)}`;
}

export function loadGamepadMapping(gamepadId: string): GamepadMapping {
  try {
    const raw = localStorage.getItem(storageKey(gamepadId));
    if (!raw) return defaultGamepadMapping(gamepadId);
    const parsed = JSON.parse(raw) as Partial<GamepadMapping>;
    if (typeof parsed.gamepadId !== "string") {
      return defaultGamepadMapping(gamepadId);
    }
    const fnButtons: Record<number, number> = {};
    if (parsed.fnButtons && typeof parsed.fnButtons === "object") {
      for (const [k, v] of Object.entries(parsed.fnButtons)) {
        const fn = Number(k);
        if (Number.isFinite(fn) && typeof v === "number") {
          fnButtons[fn] = v;
        }
      }
    }
    return {
      gamepadId,
      speedAxis:
        typeof parsed.speedAxis === "number"
          ? parsed.speedAxis
          : DEFAULT_SPEED_AXIS,
      invertAxis: parsed.invertAxis !== false,
      axisEnabled: parsed.axisEnabled !== false,
      idleAxisMin:
        typeof parsed.idleAxisMin === "number"
          ? parsed.idleAxisMin
          : undefined,
      idleAxisMax:
        typeof parsed.idleAxisMax === "number"
          ? parsed.idleAxisMax
          : undefined,
      reverseButton:
        typeof parsed.reverseButton === "number"
          ? parsed.reverseButton
          : undefined,
      stopButton:
        typeof parsed.stopButton === "number" ? parsed.stopButton : undefined,
      accelerateButton:
        typeof parsed.accelerateButton === "number"
          ? parsed.accelerateButton
          : undefined,
      decelerateButton:
        typeof parsed.decelerateButton === "number"
          ? parsed.decelerateButton
          : undefined,
      fnButtons,
      speedSensitivity: parseSpeedSensitivity(parsed.speedSensitivity),
      speedButtonSteps: parseSpeedButtonSteps(parsed.speedButtonSteps),
      enabled: parsed.enabled === true,
    };
  } catch {
    return defaultGamepadMapping(gamepadId);
  }
}

export function saveGamepadMapping(mapping: GamepadMapping): void {
  try {
    localStorage.setItem(storageKey(mapping.gamepadId), JSON.stringify(mapping));
  } catch {
    /* ignore */
  }
}

export function parseSpeedSensitivity(value: unknown): GamepadSpeedSensitivity {
  const n = typeof value === "number" ? Math.round(value) : 0;
  if (n <= 0) return 0;
  if (n >= GAMEPAD_SPEED_SENSITIVITY_MAX) return GAMEPAD_SPEED_SENSITIVITY_MAX;
  return n as GamepadSpeedSensitivity;
}

export function parseSpeedButtonSteps(value: unknown): number {
  const n =
    typeof value === "number" ? Math.round(value) : DEFAULT_SPEED_BUTTON_STEPS;
  if (n < MIN_SPEED_BUTTON_STEPS) return MIN_SPEED_BUTTON_STEPS;
  if (n > MAX_SPEED_BUTTON_STEPS) return MAX_SPEED_BUTTON_STEPS;
  return n;
}

/** Speed delta for one accelerate/decelerate button press. */
export function speedButtonStepSize(maxSpeed: number, steps?: number): number {
  return Math.max(
    1,
    Math.round(maxSpeed / parseSpeedButtonSteps(steps)),
  );
}

export function gamepadSpeedDivisor(
  sensitivity: number | undefined,
): number {
  return GAMEPAD_SPEED_SENSITIVITY_DIVISORS[parseSpeedSensitivity(sensitivity)];
}

/** Human-readable scale label for slider marks (÷1.25 … ÷3). */
export function formatSpeedSensitivityDivisor(divisor: number): string {
  return `÷${divisor}`;
}

/** Map axis value (-1..1) to speed 0..maxSpeed. */
export function axisToSpeed(
  axisValue: number,
  maxSpeed: number,
  mapping: Pick<
    GamepadMapping,
    "invertAxis" | "idleAxisMin" | "idleAxisMax" | "speedSensitivity"
  >,
  deadzone = 0.08,
): number {
  const divisor = gamepadSpeedDivisor(mapping.speedSensitivity);
  const scaledMax = maxSpeed / divisor;
  const invertAxis = mapping.invertAxis;

  if (
    typeof mapping.idleAxisMin === "number" &&
    typeof mapping.idleAxisMax === "number"
  ) {
    if (axisValue >= mapping.idleAxisMin && axisValue <= mapping.idleAxisMax) {
      return 0;
    }

    const v = invertAxis ? -axisValue : axisValue;
    const idleVMin = invertAxis ? -mapping.idleAxisMax : mapping.idleAxisMin;
    const idleVMax = invertAxis ? -mapping.idleAxisMin : mapping.idleAxisMax;

    let normalized = 0;
    if (v > idleVMax) {
      const span = 1 - idleVMax;
      normalized = span > 0 ? (v - idleVMax) / span : 0;
    } else if (v < idleVMin) {
      const span = idleVMin - -1;
      normalized = span > 0 ? (idleVMin - v) / span : 0;
    }
    return Math.round(Math.min(1, Math.max(0, normalized)) * scaledMax);
  }

  // Fallback without idle calibration: maps the full -1..1 axis range to 0..maxSpeed,
  // so a centred spring-return stick still yields ~50% speed past the deadzone.
  // The setup wizard normally learns idle range first; this path covers partial state.
  let v = invertAxis ? -axisValue : axisValue;
  if (Math.abs(v) < deadzone) {
    return 0;
  }
  const normalized = (v + 1) / 2;
  return Math.round(Math.min(1, Math.max(0, normalized)) * scaledMax);
}

export function hasIdleCalibration(
  mapping: Pick<GamepadMapping, "idleAxisMin" | "idleAxisMax">,
): boolean {
  return (
    typeof mapping.idleAxisMin === "number" &&
    typeof mapping.idleAxisMax === "number"
  );
}

/** True after the user completed warning + idle setup and left joystick enabled. */
export function isGamepadSetupComplete(
  mapping: Pick<GamepadMapping, "enabled" | "idleAxisMin" | "idleAxisMax">,
): boolean {
  return mapping.enabled === true && hasIdleCalibration(mapping);
}

/** Expand the detected idle range by a safety margin on both sides. */
export function finalizeIdleAxisRange(
  detectedMin: number,
  detectedMax: number,
): { idleAxisMin: number; idleAxisMax: number } {
  const span = detectedMax - detectedMin;
  const buffer =
    Math.max(Math.abs(detectedMin), Math.abs(detectedMax), span) *
    GAMEPAD_IDLE_MAX_BUFFER_RATIO;
  return {
    idleAxisMin: Math.max(-1, detectedMin - buffer),
    idleAxisMax: Math.min(1, detectedMax + buffer),
  };
}

export const GAMEPAD_SPEED_WINDOW_MS = 500;
export const GAMEPAD_MAX_SPEED_STEP = 20;

/** Move toward target by at most maxStep speed steps. */
export function stabilizeGamepadSpeedStep(
  appliedSpeed: number,
  targetSpeed: number,
  maxStep = GAMEPAD_MAX_SPEED_STEP,
): number {
  const delta = Math.max(
    -maxStep,
    Math.min(maxStep, targetSpeed - appliedSpeed),
  );
  return appliedSpeed + delta;
}
