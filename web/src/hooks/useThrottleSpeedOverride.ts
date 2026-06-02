import { useCallback, useEffect, useState } from "react";

/** After the user moves the throttle, ignore server speed for this long. */
export const THROTTLE_SERVER_SYNC_GRACE_MS = 2000;

interface SpeedOverride {
  speed: number;
  until: number;
}

// useThrottleSpeedOverride keeps the throttle lever on the user's value
// while dragging and for THROTTLE_SERVER_SYNC_GRACE_MS afterward, so
// loco.state echoes do not jerk the handle.
export function useThrottleSpeedOverride(
  serverSpeed: number,
  resetKey: number | null,
) {
  const [override, setOverride] = useState<SpeedOverride | null>(null);

  useEffect(() => {
    setOverride(null);
  }, [resetKey]);

  const noteUserSpeed = useCallback((speed: number) => {
    setOverride({
      speed,
      until: Date.now() + THROTTLE_SERVER_SYNC_GRACE_MS,
    });
  }, []);

  useEffect(() => {
    if (!override) {
      return;
    }
    const remaining = override.until - Date.now();
    if (remaining <= 0) {
      setOverride(null);
      return;
    }
    const timer = window.setTimeout(() => setOverride(null), remaining);
    return () => window.clearTimeout(timer);
  }, [override]);

  const inGrace =
    override != null && Date.now() < override.until;
  const displaySpeed = inGrace ? override.speed : serverSpeed;

  return { displaySpeed, noteUserSpeed, inGrace };
}
