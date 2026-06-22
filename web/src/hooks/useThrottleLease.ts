import { useMemo } from "react";

import type { LeaseEntry } from "../api/leases";
import type { ThrottleTarget } from "./useThrottleTargetSelection";

export function findLeaseForTarget(
  leases: LeaseEntry[] | undefined,
  target: ThrottleTarget | null,
  vehicleIdByAddr: Map<number, string>,
): LeaseEntry | null {
  if (!leases || !target) return null;
  if (target.kind === "train") {
    return (
      leases.find(
        (l) => l.kind === "train" && l.targetId === target.trainId,
      ) ?? null
    );
  }
  const vehicleId = vehicleIdByAddr.get(target.dccAddress);
  if (!vehicleId) return null;
  return (
    leases.find(
      (l) => l.kind === "vehicle" && l.targetId === vehicleId,
    ) ?? null
  );
}

export function effectiveLeaseMaxSpeed(
  maxSpeed: number,
  speedLimit: number | undefined,
): number {
  if (speedLimit == null || speedLimit <= 0 || speedLimit >= 100) {
    return maxSpeed;
  }
  return Math.max(1, Math.floor((maxSpeed * speedLimit) / 100));
}

export function useThrottleLease(
  leases: LeaseEntry[] | undefined,
  target: ThrottleTarget | null,
  vehicles: { id: string; dccAddress: number }[],
) {
  const idByAddr = useMemo(() => {
    const m = new Map<number, string>();
    for (const v of vehicles) {
      m.set(v.dccAddress, v.id);
    }
    return m;
  }, [vehicles]);

  return useMemo(
    () => findLeaseForTarget(leases, target, idByAddr),
    [leases, target, idByAddr],
  );
}
