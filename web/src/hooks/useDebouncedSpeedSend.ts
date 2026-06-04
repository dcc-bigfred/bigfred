import { useCallback, useEffect, useRef } from "react";

/** Delay before loco.setSpeed is sent while the throttle moves. */
export const THROTTLE_SPEED_SEND_DELAY_MS = 100;

type PendingSpeed = { address: number; speed: number; forward: boolean };

// useDebouncedSpeedSend batches rapid throttle moves into occasional
// loco.setSpeed calls. UI updates stay immediate via a separate path.
export function useDebouncedSpeedSend(
  setSpeed: (
    address: number,
    speed: number,
    forward: boolean,
  ) => Promise<{ ok: boolean; error?: string }>,
) {
  const timerRef = useRef<ReturnType<typeof setTimeout>>();
  const pendingRef = useRef<PendingSpeed | null>(null);
  const setSpeedRef = useRef(setSpeed);
  setSpeedRef.current = setSpeed;

  const flush = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = undefined;
    }
    const pending = pendingRef.current;
    if (!pending) {
      return;
    }
    pendingRef.current = null;
    void setSpeedRef.current(pending.address, pending.speed, pending.forward);
  }, []);

  const queueSpeed = useCallback(
    (address: number, speed: number, forward: boolean) => {
      pendingRef.current = { address, speed, forward };
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
      timerRef.current = setTimeout(flush, THROTTLE_SPEED_SEND_DELAY_MS);
    },
    [flush],
  );

  const sendSpeedNow = useCallback(
    (address: number, speed: number, forward: boolean) => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = undefined;
      }
      pendingRef.current = null;
      void setSpeedRef.current(address, speed, forward);
    },
    [],
  );

  useEffect(
    () => () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
      const pending = pendingRef.current;
      if (pending) {
        void setSpeedRef.current(
          pending.address,
          pending.speed,
          pending.forward,
        );
      }
    },
    [],
  );

  return { queueSpeed, sendSpeedNow, flush };
}
