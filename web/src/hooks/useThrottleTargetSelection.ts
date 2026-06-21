import { useCallback, useEffect, useState } from "react";

const STORAGE_KEY_PREFIX = "bigfred.throttle.selectedTarget";

export type ThrottleVehicleTarget = {
  kind: "vehicle";
  dccAddress: number;
};

export type ThrottleTrainTarget = {
  kind: "train";
  trainId: string;
};

export type ThrottleTarget = ThrottleVehicleTarget | ThrottleTrainTarget;

function storageKey(layoutID: number): string {
  return `${STORAGE_KEY_PREFIX}.${layoutID}`;
}

function readStoredTarget(layoutID: number): ThrottleTarget | null {
  try {
    const raw = localStorage.getItem(storageKey(layoutID));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as ThrottleTarget;
    if (parsed?.kind === "vehicle" && typeof parsed.dccAddress === "number") {
      return parsed;
    }
    if (parsed?.kind === "train" && typeof parsed.trainId === "string" && parsed.trainId.length > 0) {
      return parsed;
    }
  } catch {
    /* ignore */
  }
  return null;
}

function writeStoredTarget(layoutID: number, target: ThrottleTarget | null) {
  try {
    if (target == null) {
      localStorage.removeItem(storageKey(layoutID));
    } else {
      localStorage.setItem(storageKey(layoutID), JSON.stringify(target));
    }
  } catch {
    /* ignore */
  }
}

/** Persists the last drivable target (vehicle or train) per layout. */
export function useThrottleTargetSelection(
  layoutID: number,
  vehicleAddresses: number[],
  trainIds: string[],
) {
  const [selectedTarget, setSelectedTarget] = useState<ThrottleTarget | null>(
    null,
  );

  useEffect(() => {
    const stored = readStoredTarget(layoutID);
    if (stored?.kind === "vehicle" && vehicleAddresses.includes(stored.dccAddress)) {
      setSelectedTarget(stored);
      return;
    }
    if (stored?.kind === "train" && trainIds.includes(stored.trainId)) {
      setSelectedTarget(stored);
      return;
    }
    if (vehicleAddresses.length > 0) {
      setSelectedTarget({ kind: "vehicle", dccAddress: vehicleAddresses[0] });
      return;
    }
    if (trainIds.length > 0) {
      setSelectedTarget({ kind: "train", trainId: trainIds[0] });
      return;
    }
    setSelectedTarget(null);
  }, [layoutID, vehicleAddresses.join(","), trainIds.join(",")]);

  const selectTarget = useCallback(
    (target: ThrottleTarget) => {
      setSelectedTarget(target);
      writeStoredTarget(layoutID, target);
    },
    [layoutID],
  );

  return { selectedTarget, selectTarget };
}
