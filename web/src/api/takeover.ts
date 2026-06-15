import { useCallback } from "react";

import { useSocket } from "../context/SocketContext";

export type TakeoverTarget = "vehicle" | "train";

export interface TakeoverUser {
  userId: number;
  login: string;
}

export interface TakeoverRequestedEvent {
  requestId: number;
  signalman: TakeoverUser;
  target: TakeoverTarget;
  targetId: number;
  autoGrantAt: number;
}

export interface TakeoverGrantedEvent {
  requestId: number;
  target: TakeoverTarget;
  targetId: number;
  signalman: TakeoverUser;
  leaseExpiresAt: number;
}

export interface TakeoverReleasedEvent {
  requestId: number;
  target: TakeoverTarget;
  targetId: number;
  reason?: string;
}

export function useTakeoverActions() {
  const { sendAction } = useSocket();

  const requestTakeover = useCallback(
    (target: TakeoverTarget, targetId: number) =>
      sendAction("takeover.request", { target, targetId }),
    [sendAction],
  );

  const rejectTakeover = useCallback(
    (requestId: number) => sendAction("takeover.reject", { requestId }),
    [sendAction],
  );

  const cancelTakeover = useCallback(
    (requestId: number) => sendAction("takeover.cancel", { requestId }),
    [sendAction],
  );

  const releaseTakeover = useCallback(
    (requestId: number) => sendAction("takeover.release", { requestId }),
    [sendAction],
  );

  return { requestTakeover, rejectTakeover, cancelTakeover, releaseTakeover };
}
