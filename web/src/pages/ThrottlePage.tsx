import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import {
  useSocket,
  type CommandStationChangedPayload,
} from "../context/SocketContext";
import {
  DccBusProvider,
  useDccBus,
  useDccBusOptional,
  type DataPlaneStatus,
} from "../context/DccBusContext";
import { useMe } from "../api/auth";
import { useLayoutVehicles } from "../api/vehicles";
import ThrottleCockpit from "../components/throttle/ThrottleCockpit";
import ThrottleSetupDialog from "../components/throttle/ThrottleSetupDialog";

function translateErrorCode(
  t: (k: string, opts?: { defaultValue?: string }) => string,
  code: string,
  fallback: string,
): string {
  const key = `throttle:errors.${code}` as unknown as string;
  const resolved = t(key, { defaultValue: "" });
  return resolved && resolved !== key ? resolved : fallback;
}

function maxSpeedValue(speedSteps: number): number {
  switch (speedSteps) {
    case 14:
      return 15;
    case 28:
      return 28;
    default:
      return 127;
  }
}

export default function ThrottlePage() {
  const { session, setCommandStation, connected, reconnecting, subscribe } =
    useSocket();
  const me = useMe().data;
  const { t } = useTranslation(["throttle", "common", "errors"]);

  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, []);

  const layoutID = me?.layoutId ?? null;
  const stations = session?.availableCommandStations ?? [];
  const [selectedCS, setSelectedCS] = useState(0);
  const [selecting, setSelecting] = useState(false);
  const [spawnAcked, setSpawnAcked] = useState(false);
  const [spawnError, setSpawnError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);
  const [setupOpen, setSetupOpen] = useState(false);
  const spawnGenRef = useRef(0);

  const activeStation = useMemo(
    () => stations.find((s) => s.id === selectedCS),
    [stations, selectedCS],
  );
  const activeWsUrl = activeStation?.wsUrl ?? null;

  useEffect(() => {
    setSpawnAcked(false);
    setSpawnError(null);
  }, [session?.sessionId, selectedCS]);

  useEffect(() => {
    return subscribe("session.commandStationChanged", (raw) => {
      const p = raw as CommandStationChangedPayload;
      if (p.commandStationId !== selectedCS) {
        return;
      }
      if (p.status === "running" && p.wsUrl) {
        setSpawnAcked(true);
        setSpawnError(null);
        setSelecting(false);
      } else if (p.status === "degraded") {
        setSpawnError(p.reason ?? "dcc_bus_unavailable");
        setSelecting(false);
      }
    });
  }, [subscribe, selectedCS]);

  useEffect(() => {
    const fromSession = session?.currentSession?.commandStationId ?? 0;
    if (fromSession > 0) {
      setSelectedCS(fromSession);
    }
  }, [session?.sessionId]);

  useEffect(() => {
    if (stations.length === 1 && selectedCS === 0) {
      setSelectedCS(stations[0].id);
    }
  }, [stations, selectedCS]);

  useEffect(() => {
    if (!connected || !session?.sessionId || selectedCS <= 0) {
      return;
    }

    const gen = ++spawnGenRef.current;
    setSelecting(true);
    setSpawnError(null);

    void setCommandStation(selectedCS).then((result) => {
      if (gen !== spawnGenRef.current) {
        return;
      }
      setSelecting(false);
      if (result.ok) {
        setSpawnAcked(true);
      } else {
        setSpawnError(result.error ?? "dcc_bus_unavailable");
      }
    });
  }, [
    connected,
    session?.sessionId,
    selectedCS,
    retryTick,
    setCommandStation,
  ]);

  const handlePickerChange = useCallback((csID: number) => {
    setSpawnError(null);
    setSelectedCS(csID);
  }, []);

  const handleRetrySpawn = useCallback(() => {
    if (!connected) {
      setSpawnError("control_offline");
      return;
    }
    setSpawnError(null);
    setSpawnAcked(false);
    setRetryTick((n) => n + 1);
  }, [connected]);

  const setupPanel = (
    <>
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        <Chip
          color={connected ? "success" : "default"}
          label={t(
            connected
              ? "throttle:controlPlane.online"
              : "throttle:controlPlane.offline",
          )}
        />
      </Stack>

      {reconnecting && (
        <Alert severity="warning">
          {t("throttle:controlPlane.reconnecting")}
        </Alert>
      )}

      <CommandStationPicker
        stations={stations}
        currentID={selectedCS}
        disabled={selecting}
        onChange={handlePickerChange}
      />

      {spawnError && (
        <Alert
          severity="error"
          action={
            selectedCS > 0 ? (
              <Button color="inherit" size="small" onClick={handleRetrySpawn}>
                {t("retry")}
              </Button>
            ) : undefined
          }
        >
          {translateErrorCode(
            t as unknown as (
              k: string,
              opts?: { defaultValue?: string },
            ) => string,
            spawnError,
            t("throttle:errors.dcc_bus_unavailable"),
          )}
        </Alert>
      )}

      {selectedCS === 0 && (
        <Alert severity="info">
          {stations.length === 0
            ? t("throttle:noCommandStations")
            : t("throttle:selectCommandStation")}
        </Alert>
      )}

      {selectedCS !== 0 &&
        activeWsUrl == null &&
        !spawnError &&
        (selecting || spawnAcked) && (
          <Alert severity="info" icon={false}>
            {t("throttle:csStatus.spawning")}
          </Alert>
        )}
    </>
  );

  if (layoutID == null) {
    return null;
  }

  const closeSetup = () => setSetupOpen(false);
  const openSetup = () => setSetupOpen(true);

  const pageSx = {
    flex: 1,
    display: "flex",
    flexDirection: "column" as const,
    minHeight: 0,
    maxHeight: "100%",
    minWidth: 0,
    width: "100%",
    overflow: "hidden",
  };

  const cockpitAreaSx = {
    flex: 1,
    display: "flex",
    flexDirection: "column" as const,
    minHeight: 0,
    minWidth: 0,
  };

  return (
    <Box sx={pageSx}>
      <ThrottleSetupDialog open={setupOpen} onClose={closeSetup}>
        {setupPanel}
        <SetupDataPlaneSection />
      </ThrottleSetupDialog>

      <Box sx={cockpitAreaSx}>
        {activeWsUrl ? (
          <DccBusProvider wsUrl={activeWsUrl}>
            <ConnectedThrottle
              layoutID={layoutID}
              speedSteps={activeStation?.speedSteps ?? 128}
              onOpenSetup={openSetup}
            />
          </DccBusProvider>
        ) : (
          <IdleThrottle layoutID={layoutID} onOpenSetup={openSetup} />
        )}
      </Box>
    </Box>
  );
}

function SetupDataPlaneSection() {
  const dcc = useDccBusOptional();
  const { t } = useTranslation("throttle");

  return (
    <>
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        <DataPlaneStatusChip status={dcc?.status ?? "idle"} />
      </Stack>
      {dcc?.reconnecting && (
        <Alert severity="warning">{t("reconnecting")}</Alert>
      )}
    </>
  );
}

function ReconnectingAlert() {
  const { reconnecting } = useDccBus();
  const { t } = useTranslation("throttle");
  if (!reconnecting) {
    return null;
  }
  return <Alert severity="warning">{t("reconnecting")}</Alert>;
}

function DataPlaneStatusChip({ status }: { status: DataPlaneStatus }) {
  const { t } = useTranslation("throttle");
  switch (status) {
    case "open":
      return <Chip color="success" label={t("dataPlane.online")} />;
    case "connecting":
      return <Chip color="warning" label={t("dataPlane.connecting")} />;
    default:
      return <Chip label={t("dataPlane.offline")} />;
  }
}

function CommandStationPicker({
  stations,
  currentID,
  disabled,
  onChange,
}: {
  stations: { id: number; name: string; kind: string }[];
  currentID: number;
  disabled?: boolean;
  onChange: (id: number) => void;
}) {
  const { t } = useTranslation("throttle");
  if (stations.length === 0) return null;
  return (
    <FormControl fullWidth disabled={disabled}>
      <InputLabel id="command-station-label">{t("commandStation")}</InputLabel>
      <Select
        labelId="command-station-label"
        value={currentID > 0 ? String(currentID) : ""}
        label={t("commandStation")}
        onChange={(ev) => {
          const raw = ev.target.value;
          if (raw === "") {
            onChange(0);
            return;
          }
          const csID = Number(raw);
          if (Number.isFinite(csID) && csID > 0) {
            onChange(csID);
          }
        }}
      >
        <MenuItem value="">
          <em>—</em>
        </MenuItem>
        {stations.map((s) => (
          <MenuItem key={s.id} value={String(s.id)}>
            {s.name} ({s.kind})
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}

function useCockpitVehicles(layoutID: number) {
  const roster = useLayoutVehicles(layoutID).data ?? [];
  return useMemo(
    () =>
      roster
        .filter((v) => v.dccAddress != null)
        .map((v) => ({
          id: v.id,
          name: v.name,
          dccAddress: v.dccAddress as number,
        })),
    [roster],
  );
}

function IdleThrottle({
  layoutID,
  onOpenSetup,
}: {
  layoutID: number;
  onOpenSetup: () => void;
}) {
  const vehicles = useCockpitVehicles(layoutID);
  const [selectedAddr, setSelectedAddr] = useState<number | null>(null);

  useEffect(() => {
    if (selectedAddr == null && vehicles.length > 0) {
      setSelectedAddr(vehicles[0].dccAddress);
    }
  }, [vehicles, selectedAddr]);

  return (
    <Box sx={{ flex: 1, display: "flex", flexDirection: "column", minHeight: 0 }}>
      <ThrottleCockpit
        onOpenSetup={onOpenSetup}
        vehicles={vehicles}
        selectedAddress={selectedAddr}
        onSelectAddress={setSelectedAddr}
        speed={0}
        maxSpeed={127}
        forward
        functions={[]}
        disabled
        onSpeedChange={() => {}}
        onDirectionChange={() => {}}
        onFunctionToggle={() => {}}
        onStop={() => {}}
      />
    </Box>
  );
}

function ConnectedThrottle({
  layoutID,
  speedSteps: sessionSpeedSteps,
  onOpenSetup,
}: {
  layoutID: number;
  speedSteps: number;
  onOpenSetup: () => void;
}) {
  const vehicles = useCockpitVehicles(layoutID);
  const [selectedAddr, setSelectedAddr] = useState<number | null>(null);
  const {
    subscribe,
    status,
    states,
    lastError,
    setSpeed,
    setFunction,
    speedSteps: busSpeedSteps,
  } = useDccBus();
  const { t } = useTranslation(["throttle", "errors"]);
  const speedSteps = busSpeedSteps ?? sessionSpeedSteps;
  const maxSpeed = maxSpeedValue(speedSteps);

  const rosterAddrKey = vehicles.map((v) => v.dccAddress).join(",");
  useEffect(() => {
    if (selectedAddr == null || status !== "open") {
      return;
    }
    void subscribe([selectedAddr]);
  }, [selectedAddr, subscribe, rosterAddrKey, status]);

  useEffect(() => {
    if (selectedAddr == null && vehicles.length > 0) {
      setSelectedAddr(vehicles[0].dccAddress);
    }
  }, [vehicles, selectedAddr]);

  const state =
    selectedAddr != null ? states.get(selectedAddr) : undefined;
  const speed = state?.speed ?? 0;
  const forward = state?.forward ?? true;
  const functions = state?.functions ?? [];

  const handleSpeed = (next: number) => {
    if (selectedAddr == null) return;
    void setSpeed(selectedAddr, next, forward);
  };
  const handleDir = (fwd: boolean) => {
    if (selectedAddr == null) return;
    void setSpeed(selectedAddr, speed, fwd);
  };
  const handleFn = (n: number) => {
    if (selectedAddr == null) return;
    void setFunction(selectedAddr, n, !(functions[n] ?? false));
  };
  const handleStop = () => {
    if (selectedAddr == null) return;
    void setSpeed(selectedAddr, 0, forward);
  };

  return (
    <Box
      sx={{
        flex: 1,
        display: "flex",
        flexDirection: "column",
        minHeight: 0,
        position: "relative",
      }}
    >
      <ThrottleCockpit
        onOpenSetup={onOpenSetup}
        vehicles={vehicles}
        selectedAddress={selectedAddr}
        onSelectAddress={setSelectedAddr}
        speed={Math.min(speed, maxSpeed)}
        maxSpeed={maxSpeed}
        forward={forward}
        functions={functions}
        disabled={selectedAddr == null}
        onSpeedChange={handleSpeed}
        onDirectionChange={handleDir}
        onFunctionToggle={handleFn}
        onStop={handleStop}
      />

      <Box
        sx={{
          position: "absolute",
          left: 8,
          right: 8,
          bottom: 64,
          display: "flex",
          flexDirection: "column",
          gap: 1,
          pointerEvents: "none",
          "& .MuiAlert-root": { pointerEvents: "auto" },
        }}
      >
        <ReconnectingAlert />
        {lastError && (
          <Alert severity="warning">
            {translateErrorCode(
              t as unknown as (
                k: string,
                opts?: { defaultValue?: string },
              ) => string,
              lastError,
              t("throttle:errors.command_station_disconnected"),
            )}
          </Alert>
        )}
      </Box>
    </Box>
  );
}
