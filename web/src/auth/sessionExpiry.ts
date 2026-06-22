type SessionExpiredListener = () => void;

const listeners = new Set<SessionExpiredListener>();
let notifying = false;

/** Subscribe to session-expiry events (e.g. WebSocket 401 handshake). */
export function onSessionExpired(listener: SessionExpiredListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/**
 * Signal that the server rejected the session. Idempotent — multiple
 * concurrent WebSockets may detect expiry at once.
 */
export function notifySessionExpired(): void {
  if (notifying) return;
  notifying = true;
  for (const listener of listeners) {
    listener();
  }
}

/** Allow a fresh login cycle to detect expiry again. */
export function resetSessionExpiryGuard(): void {
  notifying = false;
}
