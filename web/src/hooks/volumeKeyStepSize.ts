/** One volume hardware key press = 5% of maxSpeed (at least 1). */
export function volumeKeyStepSize(maxSpeed: number): number {
  return Math.max(1, Math.round(maxSpeed * 0.05));
}

/** Apply a native hardware key direction to the current speed. */
export function applyThrottleHardwareKeyStep(
  direction: number,
  currentSpeed: number,
  maxSpeed: number,
): number {
  const step = volumeKeyStepSize(maxSpeed);
  const delta = direction > 0 ? step : direction < 0 ? -step : 0;
  if (delta === 0) return currentSpeed;
  return Math.min(maxSpeed, Math.max(0, currentSpeed + delta));
}
