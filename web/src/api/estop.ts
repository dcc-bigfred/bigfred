import { useCallback } from "react";

import { useSocket } from "../context/SocketContext";
import type { TakeoverTarget } from "./takeover";

export function useEstopTargetActions() {
  const { sendAction } = useSocket();

  const estopTarget = useCallback(
    (target: TakeoverTarget, targetId: number) =>
      sendAction("system.estopTarget", { target, targetId }),
    [sendAction],
  );

  return { estopTarget };
}
