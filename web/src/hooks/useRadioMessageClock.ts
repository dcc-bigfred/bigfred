import { useEffect, useState } from "react";

// Ticks every 30 s so radio chat opacity tiers update without a
// per-second render cost.
export function useRadioMessageClock(intervalMs = 30_000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
}
