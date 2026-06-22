import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { invalidateLeases } from "../api/leases";
import { useSocket } from "../context/SocketContext";

const LEASE_EVENTS = [
  "lease.created",
  "lease.updated",
  "lease.revoked",
  "lease.expired",
] as const;

export function useLeaseEvents() {
  const qc = useQueryClient();
  const { subscribe } = useSocket();

  useEffect(() => {
    const onLeaseEvent = () => {
      invalidateLeases(qc);
      // The layout roster carries the per-user `canDrive` flag that the
      // throttle uses to list (and subscribe to) drivable vehicles/trains.
      // Refreshing it here drops a lent-out vehicle from the owner's
      // throttle — so they stop driving it and their dead-man's switch no
      // longer emergency-stops the lessee — and surfaces it to the lessee,
      // mirroring the takeover flow.
      void qc.invalidateQueries({ queryKey: ["layouts"] });
    };
    const unsubs = LEASE_EVENTS.map((typ) => subscribe(typ, onLeaseEvent));
    return () => unsubs.forEach((u) => u());
  }, [subscribe, qc]);
}
