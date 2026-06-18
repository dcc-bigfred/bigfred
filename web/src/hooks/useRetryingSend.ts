import { useCallback, useEffect, useRef, useState } from "react";

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

/** Errors that mean the socket is down — retry until reconnect or deadline. */
const OFFLINE_ERRORS = new Set(["dcc_bus_offline", "control_offline"]);

// retryDelay decides how long to wait before the next attempt, or null to give
// up. Two independent budgets:
//   - ack_timeout    → count-based (the daemon is reachable but slow / dropped
//                       the frame); retry up to SPEED_RETRY_MAX times.
//   - *offline       → time-based; keep retrying every backoff until the
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
  if (res.error != null && OFFLINE_ERRORS.has(res.error)) {
    if (Date.now() >= state.deadline) return null;
    return SPEED_RETRY_BACKOFF_MS;
  }
  return null;
}

function newRetryState() {
  return {
    timeoutTriesLeft: SPEED_RETRY_MAX,
    deadline: Date.now() + RETRY_MAX_WAIT_MS,
  };
}

function runWithRetry(
  sendFn: () => Promise<SendResult>,
  state: ReturnType<typeof newRetryState>,
  isCurrent: () => boolean,
  setRetrying: (retrying: boolean) => void,
  timerRef: { current: ReturnType<typeof setTimeout> | undefined },
  onSettled?: (res: SendResult) => void,
): void {
  const attempt = () => {
    void sendFn().then((res) => {
      if (!isCurrent()) return;
      const delay = nextDelay(res, state);
      if (delay == null) {
        setRetrying(false);
        onSettled?.(res);
        return;
      }
      setRetrying(true);
      timerRef.current = setTimeout(() => {
        if (!isCurrent()) return;
        attempt();
      }, delay);
    });
  };
  attempt();
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
  const [retrying, setRetrying] = useState(false);

  const cancel = useCallback(() => {
    genRef.current += 1;
    setRetrying(false);
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = undefined;
    }
  }, []);

  const dispatch = useCallback(
    (...args: Args) => {
      cancel();
      const gen = genRef.current;
      const state = newRetryState();

      runWithRetry(
        () => sendRef.current(...args),
        state,
        () => gen === genRef.current,
        setRetrying,
        timerRef,
      );
    },
    [cancel],
  );

  // Like dispatch, but resolves once the command succeeds or retries are
  // exhausted — for actions that need to await the outcome (e.g. radio stop).
  const dispatchAsync = useCallback(
    (...args: Args): Promise<SendResult> => {
      cancel();
      const gen = genRef.current;
      const state = newRetryState();

      return new Promise((resolve) => {
        runWithRetry(
          () => sendRef.current(...args),
          state,
          () => gen === genRef.current,
          setRetrying,
          timerRef,
          resolve,
        );
      });
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

  return { dispatch, dispatchAsync, cancel, retrying };
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
  const retryKeysRef = useRef(new Set<string>());
  const sendRef = useRef(send);
  sendRef.current = send;
  const keyOfRef = useRef(keyOf);
  keyOfRef.current = keyOf;
  const [retrying, setRetrying] = useState(false);

  const syncRetrying = useCallback(() => {
    setRetrying(retryKeysRef.current.size > 0);
  }, []);

  const setKeyRetrying = useCallback(
    (key: string, active: boolean) => {
      if (active) {
        retryKeysRef.current.add(key);
      } else {
        retryKeysRef.current.delete(key);
      }
      syncRetrying();
    },
    [syncRetrying],
  );

  const dispatch = useCallback(
    (...args: Args) => {
      const key = keyOfRef.current(...args);
      const gen = (genRef.current.get(key) ?? 0) + 1;
      genRef.current.set(key, gen);
      const existing = timersRef.current.get(key);
      if (existing) {
        clearTimeout(existing);
        timersRef.current.delete(key);
      }
      retryKeysRef.current.delete(key);
      syncRetrying();
      const state = newRetryState();
      const timerSlot = {
        get current() {
          return timersRef.current.get(key);
        },
        set current(value: ReturnType<typeof setTimeout> | undefined) {
          if (value === undefined) {
            timersRef.current.delete(key);
          } else {
            timersRef.current.set(key, value);
          }
        },
      };

      runWithRetry(
        () => sendRef.current(...args),
        state,
        () => gen === genRef.current.get(key),
        (active) => setKeyRetrying(key, active),
        timerSlot,
      );
    },
    [setKeyRetrying, syncRetrying],
  );

  useEffect(() => {
    const timers = timersRef.current;
    return () => {
      for (const timer of timers.values()) {
        clearTimeout(timer);
      }
      timers.clear();
      retryKeysRef.current.clear();
    };
  }, []);

  return { dispatch, retrying };
}
