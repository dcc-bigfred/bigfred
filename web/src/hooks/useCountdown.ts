import { useEffect, useState } from "react";

// useCountdown returns a `mm:ss` string counting down from `now()`
// to the supplied `deadline`. When `deadline` is null, undefined or
// already in the past, the hook returns null so callers can hide
// the indicator entirely.
//
// The internal interval ticks every 500 ms — short enough that the
// label feels responsive, long enough to keep the React tree quiet
// when many countdowns are alive (one per AppBar icon).
export function useCountdown(deadline: Date | string | null | undefined): string | null {
  const target = deadline
    ? typeof deadline === "string"
      ? new Date(deadline)
      : deadline
    : null;

  const [, setTick] = useState(0);

  useEffect(() => {
    if (!target) return;
    const id = window.setInterval(() => setTick((t) => t + 1), 500);
    return () => window.clearInterval(id);
  }, [target?.getTime()]);

  if (!target) return null;

  const remainingMs = target.getTime() - Date.now();
  if (remainingMs <= 0) return null;

  const totalSeconds = Math.floor(remainingMs / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes.toString().padStart(2, "0")}:${seconds.toString().padStart(2, "0")}`;
}
