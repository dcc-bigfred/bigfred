import { useCallback, useEffect, useRef } from "react";

import { useRetryingSend } from "./useRetryingSend";

const DEBOUNCE_MS = 120;

type TrainSpeedSender = (
  trainId: number,
  speed: number,
  forward: boolean,
) => Promise<{ ok: boolean; error?: string }>;

/**
 * Debounces train.setSpeed calls while the slider is dragged. Each send is
 * retried on ack timeout (see useRetryingSend); a newer move cancels the
 * retry chain of the previous one.
 */
export function useDebouncedTrainSpeedSend(sendTrainSpeed: TrainSpeedSender) {
  const timerRef = useRef<number | null>(null);
  const pendingRef = useRef<{
    trainId: number;
    speed: number;
    forward: boolean;
  } | null>(null);
  const { dispatch, cancel } = useRetryingSend(sendTrainSpeed);

  const flush = useCallback(() => {
    if (timerRef.current != null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    const pending = pendingRef.current;
    pendingRef.current = null;
    if (pending) {
      dispatch(pending.trainId, pending.speed, pending.forward);
    }
  }, [dispatch]);

  const sendSpeedNow = useCallback(
    (trainId: number, speed: number, forward: boolean) => {
      if (timerRef.current != null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      pendingRef.current = null;
      dispatch(trainId, speed, forward);
    },
    [dispatch],
  );

  const queueSpeed = useCallback(
    (trainId: number, speed: number, forward: boolean) => {
      // Stop retrying the previous move immediately, even before this one
      // flushes, so a stale speed cannot land on top of the new target.
      cancel();
      pendingRef.current = { trainId, speed, forward };
      if (timerRef.current != null) {
        window.clearTimeout(timerRef.current);
      }
      timerRef.current = window.setTimeout(() => {
        timerRef.current = null;
        const p = pendingRef.current;
        pendingRef.current = null;
        if (p) {
          dispatch(p.trainId, p.speed, p.forward);
        }
      }, DEBOUNCE_MS);
    },
    [dispatch, cancel],
  );

  useEffect(
    () => () => {
      if (timerRef.current != null) {
        window.clearTimeout(timerRef.current);
      }
    },
    [],
  );

  return { queueSpeed, sendSpeedNow, flush };
}
