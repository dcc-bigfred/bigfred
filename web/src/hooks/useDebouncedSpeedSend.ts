import { useCallback, useEffect, useRef } from "react";

import { useRetryingSend } from "./useRetryingSend";

/** Delay before loco.setSpeed is sent while the throttle moves. */
export const THROTTLE_SPEED_SEND_DELAY_MS = 100;

type PendingSpeed = { address: number; speed: number; forward: boolean };

// useDebouncedSpeedSend batches rapid throttle moves into occasional
// loco.setSpeed calls. UI updates stay immediate via a separate path.
//
// Each send is retried on ack timeout (see useRetryingSend); a newer move
// cancels the retry chain of the previous one.
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
  const { dispatch, cancel, retrying } = useRetryingSend(setSpeed);

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
    dispatch(pending.address, pending.speed, pending.forward);
  }, [dispatch]);

  const queueSpeed = useCallback(
    (address: number, speed: number, forward: boolean) => {
      // Stop retrying the previous move immediately, even before this one
      // flushes, so a stale speed cannot land on top of the new target.
      cancel();
      pendingRef.current = { address, speed, forward };
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
      timerRef.current = setTimeout(flush, THROTTLE_SPEED_SEND_DELAY_MS);
    },
    [flush, cancel],
  );

  const sendSpeedNow = useCallback(
    (address: number, speed: number, forward: boolean) => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = undefined;
      }
      pendingRef.current = null;
      dispatch(address, speed, forward);
    },
    [dispatch],
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

  return { queueSpeed, sendSpeedNow, flush, retrying };
}
