import { useCallback, useEffect, useRef } from "react";

const DEBOUNCE_MS = 120;

type TrainSpeedSender = (
  trainId: number,
  speed: number,
  forward: boolean,
) => Promise<{ ok: boolean; error?: string }>;

/** Debounces train.setSpeed calls while the slider is dragged. */
export function useDebouncedTrainSpeedSend(sendTrainSpeed: TrainSpeedSender) {
  const timerRef = useRef<number | null>(null);
  const pendingRef = useRef<{
    trainId: number;
    speed: number;
    forward: boolean;
  } | null>(null);

  const flush = useCallback(() => {
    if (timerRef.current != null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    const pending = pendingRef.current;
    pendingRef.current = null;
    if (pending) {
      void sendTrainSpeed(pending.trainId, pending.speed, pending.forward);
    }
  }, [sendTrainSpeed]);

  const sendSpeedNow = useCallback(
    (trainId: number, speed: number, forward: boolean) => {
      if (timerRef.current != null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      pendingRef.current = null;
      void sendTrainSpeed(trainId, speed, forward);
    },
    [sendTrainSpeed],
  );

  const queueSpeed = useCallback(
    (trainId: number, speed: number, forward: boolean) => {
      pendingRef.current = { trainId, speed, forward };
      if (timerRef.current != null) {
        window.clearTimeout(timerRef.current);
      }
      timerRef.current = window.setTimeout(() => {
        timerRef.current = null;
        const p = pendingRef.current;
        pendingRef.current = null;
        if (p) {
          void sendTrainSpeed(p.trainId, p.speed, p.forward);
        }
      }, DEBOUNCE_MS);
    },
    [sendTrainSpeed],
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
