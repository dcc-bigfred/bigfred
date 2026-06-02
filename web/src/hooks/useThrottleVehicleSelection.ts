import { useCallback, useEffect, useState } from "react";

const STORAGE_KEY_PREFIX = "bigfred.throttle.selectedDccAddress";

function storageKey(layoutID: number): string {
  return `${STORAGE_KEY_PREFIX}.${layoutID}`;
}

function readStoredAddress(layoutID: number): number | null {
  try {
    const raw = localStorage.getItem(storageKey(layoutID));
    if (raw == null) {
      return null;
    }
    const address = Number(raw);
    return Number.isFinite(address) && address > 0 ? address : null;
  } catch {
    return null;
  }
}

function writeStoredAddress(layoutID: number, address: number): void {
  try {
    localStorage.setItem(storageKey(layoutID), String(address));
  } catch {
    // Quota or private mode — ignore.
  }
}

function resolveAddress(
  layoutID: number,
  vehicles: { dccAddress: number }[],
  current: number | null,
): number | null {
  if (vehicles.length === 0) {
    return null;
  }
  if (current != null && vehicles.some((v) => v.dccAddress === current)) {
    return current;
  }
  const stored = readStoredAddress(layoutID);
  if (stored != null && vehicles.some((v) => v.dccAddress === stored)) {
    return stored;
  }
  return vehicles[0].dccAddress;
}

// useThrottleVehicleSelection restores the last DCC address picked on
// this layout (localStorage) when the vehicle is still on the roster.
export function useThrottleVehicleSelection(
  layoutID: number,
  vehicles: { dccAddress: number }[],
) {
  const [selectedAddr, setSelectedAddr] = useState<number | null>(null);
  const rosterKey = vehicles.map((v) => v.dccAddress).join(",");

  useEffect(() => {
    setSelectedAddr((current) => resolveAddress(layoutID, vehicles, current));
  }, [layoutID, rosterKey, vehicles]);

  const selectAddress = useCallback(
    (address: number) => {
      setSelectedAddr(address);
      writeStoredAddress(layoutID, address);
    },
    [layoutID],
  );

  return { selectedAddr, selectAddress };
}
