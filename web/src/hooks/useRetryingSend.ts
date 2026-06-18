import { useCallback, useEffect, useRef } from "react";

/** Backoff between retry attempts. */
export const SPEED_RETRY_BACKOFF_MS = 200;
/** Extra attempts after the first send when the daemon does not ack in time. */
export const SPEED_RETRY_MAX = 2;
/**
 * How long we keep re-sending a command while the WebSocket is down, waiting
 * for it to reconnect. Bounded so a long outage (e.g. server restart) does not
 * replay a stale intent minutes later — only a fresh-enough one lands.
 */
export const RETRY_MAX_WAIT_MS = 3_000;

type SendResult = { ok: boolean; error?: string };

// retryDelay decides how long to wait before the next attempt, or null to give
// up. Two independent budgets:
//   - ack_timeout    → count-based (the daemon is reachable but slow / dropped
//                       the frame); retry up to SPEED_RETRY_MAX times.
//   - dcc_bus_offline → time-based; keep retrying every backoff until the
//                       socket reconnects, capped by RETRY_MAX_WAIT_MS.
// Any other error is a definitive failure and is surfaced as-is.
function nextDelay(
  res: SendResult,
  state: { timeoutTriesLeft: number; deadline: number },
): number | null {
  if (res.ok) return null;
  if (res.error === "ack_timeout") {
    if (state.timeoutTriesLeft <= 0) return null;
    state.timeoutTriesLeft -= 1;
    return SPEED_RETRY_BACKOFF_MS;
  }
  if (res.error === "dcc_bus_offline") {
    if (Date.now() >= state.deadline) return null;
    return SPEED_RETRY_BACKOFF_MS;
  }
  return null;
}

// useRetryingSend wraps a fire-and-forget sender and retries it when the daemon
// does not ack in time, or while the data-plane socket is down (waiting for it
// to reconnect, see RETRY_MAX_WAIT_MS).
//
// Each dispatch supersedes the previous one: starting a new send (e.g. a fresh
// throttle move) cancels any retry chain still pending from the prior request,
// so a stale speed is never re-sent on top of a newer one.
export function useRetryingSend<Args extends unknown[]>(
  send: (...args: Args) => Promise<SendResult>,
) {
  const genRef = useRef(0);
  const timerRef = useRef<ReturnType<typeof setTimeout>>();
  const sendRef = useRef(send);
  sendRef.current = send;

  const cancel = useCallback(() => {
    genRef.current += 1;
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = undefined;
    }
  }, []);

  const dispatch = useCallback(
    (...args: Args) => {
      cancel();
      const gen = genRef.current;
      const state = {
        timeoutTriesLeft: SPEED_RETRY_MAX,
        deadline: Date.now() + RETRY_MAX_WAIT_MS,
      };

      const attempt = () => {
        void sendRef.current(...args).then((res) => {
          if (gen !== genRef.current) return;
          const delay = nextDelay(res, state);
          if (delay == null) return;
          timerRef.current = setTimeout(() => {
            if (gen !== genRef.current) return;
            attempt();
          }, delay);
        });
      };

      attempt();
    },
    [cancel],
  );

  useEffect(
    () => () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    },
    [],
  );

  return { dispatch, cancel };
}

// useKeyedRetryingSend is the multi-stream variant of useRetryingSend. Each
// distinct key (e.g. one DCC address + function number) has its own retry
// chain, so retrying a timed-out function toggle does not interfere with, or
// get cancelled by, a toggle of a different function. Re-dispatching the same
// key supersedes its own in-flight retry.
export function useKeyedRetryingSend<Args extends unknown[]>(
  send: (...args: Args) => Promise<SendResult>,
  keyOf: (...args: Args) => string,
) {
  const genRef = useRef(new Map<string, number>());
  const timersRef = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  const sendRef = useRef(send);
  sendRef.current = send;
  const keyOfRef = useRef(keyOf);
  keyOfRef.current = keyOf;

  const dispatch = useCallback((...args: Args) => {
    const key = keyOfRef.current(...args);
    const gen = (genRef.current.get(key) ?? 0) + 1;
    genRef.current.set(key, gen);
    const existing = timersRef.current.get(key);
    if (existing) {
      clearTimeout(existing);
      timersRef.current.delete(key);
    }
    const state = {
      timeoutTriesLeft: SPEED_RETRY_MAX,
      deadline: Date.now() + RETRY_MAX_WAIT_MS,
    };

    const attempt = () => {
      void sendRef.current(...args).then((res) => {
        if (gen !== genRef.current.get(key)) return;
        const delay = nextDelay(res, state);
        if (delay == null) return;
        const timer = setTimeout(() => {
          if (gen !== genRef.current.get(key)) return;
          attempt();
        }, delay);
        timersRef.current.set(key, timer);
      });
    };

    attempt();
  }, []);

  useEffect(() => {
    const timers = timersRef.current;
    return () => {
      for (const timer of timers.values()) {
        clearTimeout(timer);
      }
      timers.clear();
    };
  }, []);

  return { dispatch };
}
