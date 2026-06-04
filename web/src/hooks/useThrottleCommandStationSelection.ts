import { useCallback, useEffect, useState } from "react";

const STORAGE_KEY_PREFIX = "bigfred.throttle.selectedCommandStationId";

function storageKey(layoutID: number): string {
  return `${STORAGE_KEY_PREFIX}.${layoutID}`;
}

function readStoredCommandStationId(layoutID: number): number {
  try {
    const raw = localStorage.getItem(storageKey(layoutID));
    if (raw == null) {
      return 0;
    }
    const id = Number(raw);
    return Number.isFinite(id) && id > 0 ? id : 0;
  } catch {
    return 0;
  }
}

function writeStoredCommandStationId(layoutID: number, commandStationId: number): void {
  try {
    localStorage.setItem(storageKey(layoutID), String(commandStationId));
  } catch {
    // Quota or private mode — ignore.
  }
}

function resolveCommandStationId(
  layoutID: number,
  stations: { id: number }[],
  current: number,
  sessionCommandStationId: number,
): number {
  if (stations.length === 0) {
    return 0;
  }
  if (current > 0 && stations.some((s) => s.id === current)) {
    return current;
  }
  const stored = readStoredCommandStationId(layoutID);
  if (stored > 0 && stations.some((s) => s.id === stored)) {
    return stored;
  }
  if (
    sessionCommandStationId > 0 &&
    stations.some((s) => s.id === sessionCommandStationId)
  ) {
    return sessionCommandStationId;
  }
  if (stations.length === 1) {
    return stations[0].id;
  }
  return 0;
}

// useThrottleCommandStationSelection restores the last command station
// picked on this layout (localStorage) when it is still available.
export function useThrottleCommandStationSelection(
  layoutID: number,
  stations: { id: number }[],
  sessionCommandStationId: number,
) {
  const [selectedCS, setSelectedCS] = useState(0);
  const stationsKey = stations.map((s) => s.id).join(",");

  useEffect(() => {
    setSelectedCS((current) =>
      resolveCommandStationId(
        layoutID,
        stations,
        current,
        sessionCommandStationId,
      ),
    );
  }, [layoutID, stationsKey, stations, sessionCommandStationId]);

  const selectCommandStation = useCallback(
    (commandStationId: number) => {
      setSelectedCS(commandStationId);
      if (commandStationId > 0) {
        writeStoredCommandStationId(layoutID, commandStationId);
      }
    },
    [layoutID],
  );

  return { selectedCS, selectCommandStation };
}
