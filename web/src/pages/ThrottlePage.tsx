import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  Stack,
  Typography,
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
import { useVehicleFunctions } from "../api/functions";
import {
  useLayoutTrains,
  useLayoutVehicles,
  usePatchTrainMemberMultiplier,
  type RosterTrain,
} from "../api/vehicles";
import type { ThrottleCockpitFunction } from "../components/throttle/ThrottleCockpit";
import AutoDismissAlert from "../components/AutoDismissAlert";
import ThrottleCockpit from "../components/throttle/ThrottleCockpit";
import TrainFunctionAccordions from "../components/throttle/TrainFunctionAccordions";
import TrainMemberSettingsDialog from "../components/throttle/TrainMemberSettingsDialog";
import ThrottleNavigationGuard from "../components/throttle/ThrottleNavigationGuard";
import ThrottleSetupDialog from "../components/throttle/ThrottleSetupDialog";
import CommandStationPicker from "../components/throttle/CommandStationPicker";
import { useDebouncedSpeedSend } from "../hooks/useDebouncedSpeedSend";
import { useRadioStopSound } from "../hooks/useRadioStopSound";
import { useThrottleSpeedOverride } from "../hooks/useThrottleSpeedOverride";
import { useThrottleCommandStationSelection } from "../hooks/useThrottleCommandStationSelection";
import { useDebouncedTrainSpeedSend } from "../hooks/useDebouncedTrainSpeedSend";
import { useThrottleTargetSelection } from "../hooks/useThrottleTargetSelection";
import { useTrainAccordionExpanded } from "../hooks/useTrainAccordionExpanded";
import { useDriverRadioInbound } from "../hooks/useDriverRadioInbound";
import { buildThrottleRadioHeader } from "../hooks/useThrottleRadioChat";
import type { DriverRadioInbound } from "../hooks/useDriverRadioInbound";
import {
  TakeoverDriverDialog,
  useTakeoverDriverSession,
} from "../components/throttle/TakeoverDriverDialog";

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
  useRadioStopSound();
  const driverRadio = useDriverRadioInbound();
  const takeoverDriver = useTakeoverDriverSession(layoutID);
  const stations = session?.availableCommandStations ?? [];
  const sessionCommandStationId =
    session?.currentSession?.commandStationId ?? 0;
  const { selectedCS, selectCommandStation } = useThrottleCommandStationSelection(
    layoutID ?? 0,
    stations,
    sessionCommandStationId,
  );
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

  const handlePickerChange = useCallback(
    (csID: number) => {
      setSpawnError(null);
      selectCommandStation(csID);
    },
    [selectCommandStation],
  );

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
        <AutoDismissAlert severity="warning" resetKey="control-reconnecting">
          {t("throttle:controlPlane.reconnecting")}
        </AutoDismissAlert>
      )}

      <CommandStationPicker
        stations={stations}
        currentID={selectedCS}
        disabled={selecting}
        allowClear
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
        <AutoDismissAlert
          severity="info"
          resetKey={`select-cs-${stations.length}`}
        >
          {stations.length === 0
            ? t("throttle:noCommandStations")
            : t("throttle:selectCommandStation")}
        </AutoDismissAlert>
      )}

      {selectedCS !== 0 &&
        activeWsUrl == null &&
        !spawnError &&
        (selecting || spawnAcked) && (
          <AutoDismissAlert severity="info" icon={false} resetKey="spawning">
            {t("throttle:csStatus.spawning")}
          </AutoDismissAlert>
        )}
    </>
  );

  if (layoutID == null) {
    return null;
  }

  const closeSetup = () => setSetupOpen(false);
  const openSetup = () => setSetupOpen(true);

  const setupDialog = (
    <ThrottleSetupDialog open={setupOpen} onClose={closeSetup}>
      {setupPanel}
      <SetupDataPlaneSection />
    </ThrottleSetupDialog>
  );

  const pageSx = {
    flex: 1,
    display: "flex",
    flexDirection: "column" as const,
    minHeight: 0,
    maxHeight: "100%",
    minWidth: 0,
    width: "100%",
    overflow: "hidden",
    userSelect: "none",
    WebkitUserSelect: "none",
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
      {driverRadio.alertNode}
      {driverRadio.overlay}
      <TakeoverDriverDialog
        pending={takeoverDriver.pending}
        onDismiss={takeoverDriver.dismissPending}
      />
      {takeoverDriver.evictionToast && (
        <AutoDismissAlert severity="warning" resetKey="takeover-eviction">
          {t("throttle:takeover.evicted")}
        </AutoDismissAlert>
      )}
      <Box sx={cockpitAreaSx}>
        {activeWsUrl ? (
          <DccBusProvider wsUrl={activeWsUrl}>
            {setupDialog}
            <ConnectedThrottle
              layoutID={layoutID}
              speedSteps={activeStation?.speedSteps ?? 128}
              onOpenSetup={openSetup}
              driverRadio={driverRadio}
            />
          </DccBusProvider>
        ) : (
          <>
            {setupDialog}
            <IdleThrottle
              layoutID={layoutID}
              onOpenSetup={openSetup}
              driverRadio={driverRadio}
            />
          </>
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
      {dcc?.status === "open" && (
        <Typography variant="body2" color="text.secondary">
          {t("dataPlane.ping")}:{" "}
          {dcc.pingLatencyMs != null
            ? t("dataPlane.pingMs", { ms: Math.round(dcc.pingLatencyMs) })
            : t("dataPlane.pingMeasuring")}
        </Typography>
      )}
      {dcc?.reconnecting && (
        <AutoDismissAlert severity="warning" resetKey="dcc-reconnecting">
          {t("reconnecting")}
        </AutoDismissAlert>
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
  return (
    <AutoDismissAlert severity="warning" resetKey="cockpit-reconnecting">
      {t("reconnecting")}
    </AutoDismissAlert>
  );
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

function useCockpitVehicles(layoutID: number) {
  const roster = useLayoutVehicles(layoutID).data ?? [];
  return useMemo(
    () =>
      roster
        .filter((v) => v.dccAddress != null && v.canDrive !== false)
        .map((v) => ({
          id: v.id,
          name: v.name,
          dccAddress: v.dccAddress as number,
        })),
    [roster],
  );
}

function useCockpitConfiguredFunctions(
  vehicles: { id: number; dccAddress: number }[],
  selectedAddr: number | null,
): ThrottleCockpitFunction[] {
  const vehicleId =
    selectedAddr != null
      ? vehicles.find((v) => v.dccAddress === selectedAddr)?.id
      : undefined;
  const fnList = useVehicleFunctions(vehicleId ?? 0).data ?? [];
  return useMemo(
    () =>
      [...fnList]
        .sort((a, b) => a.position - b.position)
        .map((f) => ({
          num: f.num,
          label: f.name,
          icon: f.icon,
        })),
    [fnList],
  );
}

function useSelectedDriveContext(
  layoutID: number,
  selectedAddr: number | null,
): { vehicleId: number | null; vehicleName: string | null } {
  const roster = useLayoutVehicles(layoutID).data ?? [];
  const vehicle =
    selectedAddr != null
      ? roster.find((v) => v.dccAddress === selectedAddr)
      : undefined;
  return {
    vehicleId: vehicle?.id ?? null,
    vehicleName: vehicle?.name ?? null,
  };
}


function useCockpitTrains(layoutID: number) {
  const roster = useLayoutTrains(layoutID).data ?? [];
  return useMemo(
    () =>
      roster
        .filter((tr) => tr.canDrive !== false)
        .map((tr) => ({ id: tr.id, name: tr.name })),
    [roster],
  );
}

function findLeadingMember(train: RosterTrain, vehiclesById: Map<number, { name: string; dccAddress: number | null }>) {
  const sorted = [...train.members].sort((a, b) => a.position - b.position);
  for (const m of sorted) {
    const v = vehiclesById.get(m.vehicleId);
    if (v?.dccAddress != null) {
      return { member: m, vehicle: v, dccAddress: v.dccAddress };
    }
  }
  return null;
}

function useSelectedTrainContext(layoutID: number, trainId: number | null) {
  const trains = useLayoutTrains(layoutID).data ?? [];
  const vehicles = useLayoutVehicles(layoutID).data ?? [];
  return useMemo(() => {
    if (trainId == null) {
      return {
        train: null as RosterTrain | null,
        leadingAddr: null as number | null,
        leadingName: null as string | null,
        poweredMembers: [] as Array<{
          memberId: number;
          vehicleId: number;
          name: string;
          dccAddress: number;
          isLeading: boolean;
          speedMultiplier: number;
        }>,
      };
    }
    const train = trains.find((tr) => tr.id === trainId) ?? null;
    if (!train) {
      return {
        train: null,
        leadingAddr: null,
        leadingName: null,
        poweredMembers: [],
      };
    }
    const byId = new Map(
      vehicles.map((v) => [v.id, { name: v.name, dccAddress: v.dccAddress }]),
    );
    const leading = findLeadingMember(train, byId);
    const sorted = [...train.members].sort((a, b) => a.position - b.position);
    const poweredMembers = sorted.flatMap((m) => {
      const v = byId.get(m.vehicleId);
      if (v?.dccAddress == null) return [];
      const mult = m.speedMultiplier > 0 ? m.speedMultiplier : 1;
      return [
        {
          memberId: m.id,
          vehicleId: m.vehicleId,
          name: v.name,
          dccAddress: v.dccAddress,
          isLeading: leading?.member.id === m.id,
          speedMultiplier: mult,
        },
      ];
    });
    return {
      train,
      leadingAddr: leading?.dccAddress ?? null,
      leadingName: leading?.vehicle.name ?? null,
      poweredMembers,
    };
  }, [trainId, trains, vehicles]);
}


function IdleThrottle({
  layoutID,
  onOpenSetup,
  driverRadio,
}: {
  layoutID: number;
  onOpenSetup: () => void;
  driverRadio: DriverRadioInbound;
}) {
  const vehicles = useCockpitVehicles(layoutID);
  const trains = useCockpitTrains(layoutID);
  const { selectedTarget, selectTarget } = useThrottleTargetSelection(
    layoutID,
    vehicles.map((v) => v.dccAddress),
    trains.map((tr) => tr.id),
  );
  const selectedAddr =
    selectedTarget?.kind === "vehicle" ? selectedTarget.dccAddress : null;
  const configuredFunctions = useCockpitConfiguredFunctions(
    vehicles,
    selectedAddr,
  );
  const drive = useSelectedDriveContext(layoutID, selectedAddr);
  const headerExtra = buildThrottleRadioHeader({
    layoutId: layoutID,
    vehicleId: drive.vehicleId,
    vehicleName: drive.vehicleName,
    radio: driverRadio,
  });

  return (
    <Box sx={{ flex: 1, display: "flex", flexDirection: "column", minHeight: 0 }}>
      <ThrottleCockpit
        layoutId={layoutID}
        onOpenSetup={onOpenSetup}
        vehicles={vehicles}
        trains={trains}
        selectedTarget={selectedTarget}
        onSelectTarget={selectTarget}
        speed={0}
        maxSpeed={127}
        forward
        functions={[]}
        configuredFunctions={configuredFunctions}
        disabled
        headerExtra={headerExtra}
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
  driverRadio,
}: {
  layoutID: number;
  speedSteps: number;
  onOpenSetup: () => void;
  driverRadio: DriverRadioInbound;
}) {
  const me = useMe().data;
  const vehicles = useCockpitVehicles(layoutID);
  const trains = useCockpitTrains(layoutID);
  const { selectedTarget, selectTarget } = useThrottleTargetSelection(
    layoutID,
    vehicles.map((v) => v.dccAddress),
    trains.map((tr) => tr.id),
  );
  const isTrainMode = selectedTarget?.kind === "train";
  const selectedAddr =
    selectedTarget?.kind === "vehicle" ? selectedTarget.dccAddress : null;
  const trainCtx = useSelectedTrainContext(
    layoutID,
    isTrainMode ? selectedTarget.trainId : null,
  );
  const configuredFunctions = useCockpitConfiguredFunctions(
    vehicles,
    selectedAddr,
  );
  const {
    subscribe,
    status,
    states,
    lastError,
    setSpeed,
    setTrainSpeed,
    setFunction,
    speedSteps: busSpeedSteps,
  } = useDccBus();
  const { t } = useTranslation(["throttle", "errors"]);
  const speedSteps = busSpeedSteps ?? sessionSpeedSteps;
  const maxSpeed = maxSpeedValue(speedSteps);

  const subscribeAddrs = useMemo(() => {
    if (isTrainMode) {
      return trainCtx.poweredMembers.map((m) => m.dccAddress);
    }
    if (selectedAddr != null) return [selectedAddr];
    return [];
  }, [isTrainMode, trainCtx.poweredMembers, selectedAddr]);

  const rosterAddrKey = subscribeAddrs.join(",");
  useEffect(() => {
    if (subscribeAddrs.length === 0 || status !== "open") return;
    void subscribe(subscribeAddrs);
  }, [subscribeAddrs, subscribe, rosterAddrKey, status]);

  const witnessAddr = isTrainMode ? trainCtx.leadingAddr : selectedAddr;
  const state = witnessAddr != null ? states.get(witnessAddr) : undefined;
  const serverSpeed = state?.speed ?? 0;
  const forward = state?.forward ?? true;
  const functions = state?.functions ?? [];

  const { displaySpeed, noteUserSpeed } = useThrottleSpeedOverride(
    serverSpeed,
    witnessAddr,
  );
  const cockpitSpeed = Math.min(displaySpeed, maxSpeed);
  const { queueSpeed, sendSpeedNow, flush } = useDebouncedSpeedSend(setSpeed);
  const {
    queueSpeed: queueTrainSpeed,
    sendSpeedNow: sendTrainSpeedNow,
    flush: flushTrain,
  } = useDebouncedTrainSpeedSend(setTrainSpeed);
  const isMoving = cockpitSpeed > 0 && witnessAddr != null;

  const [settingsMemberId, setSettingsMemberId] = useState<number | null>(null);
  const patchMultiplier = usePatchTrainMemberMultiplier();
  const { expandedMemberIds, toggleMember } = useTrainAccordionExpanded(
    isTrainMode ? selectedTarget.trainId : null,
  );

  const handleSpeed = (next: number) => {
    if (isTrainMode) {
      noteUserSpeed(next);
      queueTrainSpeed(selectedTarget.trainId, next, forward);
      return;
    }
    if (selectedAddr == null) return;
    noteUserSpeed(next);
    queueSpeed(selectedAddr, next, forward);
  };
  const handleDir = (fwd: boolean) => {
    if (isTrainMode) {
      sendTrainSpeedNow(selectedTarget.trainId, cockpitSpeed, fwd);
      return;
    }
    if (selectedAddr == null) return;
    sendSpeedNow(selectedAddr, cockpitSpeed, fwd);
  };
  const handleFn = (n: number) => {
    if (selectedAddr == null) return;
    void setFunction(selectedAddr, n, !(functions[n] ?? false));
  };
  const handleTrainFn = (memberId: number, fn: number) => {
    const member = trainCtx.poweredMembers.find((m) => m.memberId === memberId);
    if (!member) return;
    const memberState = states.get(member.dccAddress);
    const memberFns = memberState?.functions ?? [];
    void setFunction(member.dccAddress, fn, !(memberFns[fn] ?? false));
  };
  const handleStop = () => {
    noteUserSpeed(0);
    if (isTrainMode) {
      sendTrainSpeedNow(selectedTarget.trainId, 0, forward);
      return;
    }
    if (selectedAddr == null) return;
    sendSpeedNow(selectedAddr, 0, forward);
  };

  const handleLeaveConfirm = useCallback(async () => {
    flush();
    flushTrain();
    noteUserSpeed(0);
    if (isTrainMode) {
      await setTrainSpeed(selectedTarget.trainId, 0, forward);
      return;
    }
    if (selectedAddr == null) return;
    await setSpeed(selectedAddr, 0, forward, true);
  }, [
    isTrainMode,
    selectedTarget,
    selectedAddr,
    flush,
    flushTrain,
    noteUserSpeed,
    setSpeed,
    setTrainSpeed,
    forward,
  ]);

  const drive = useSelectedDriveContext(layoutID, witnessAddr);
  const headerExtra = buildThrottleRadioHeader({
    layoutId: layoutID,
    vehicleId: drive.vehicleId,
    vehicleName: drive.vehicleName ?? trainCtx.leadingName,
    radio: driverRadio,
  });

  const settingsMember =
    settingsMemberId != null
      ? trainCtx.poweredMembers.find((m) => m.memberId === settingsMemberId)
      : undefined;

  const trainAccordion = isTrainMode ? (
    <TrainFunctionAccordions
      members={trainCtx.poweredMembers}
      states={states}
      expandedMemberIds={expandedMemberIds}
      onToggleExpanded={toggleMember}
      onFunctionToggle={handleTrainFn}
      onOpenSettings={setSettingsMemberId}
      showMultiplierCog={trainCtx.train?.ownerId === me?.id}
      disabled={trainCtx.poweredMembers.length === 0}
    />
  ) : undefined;

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
      <ThrottleNavigationGuard
        active={isMoving}
        onLeaveConfirm={handleLeaveConfirm}
      />
      <TrainMemberSettingsDialog
        open={settingsMember != null}
        memberName={settingsMember?.name ?? ""}
        isLeading={settingsMember?.isLeading ?? false}
        initialMultiplier={settingsMember?.speedMultiplier ?? 1}
        saving={patchMultiplier.isPending}
        onClose={() => setSettingsMemberId(null)}
        onSave={(multiplier) => {
          if (!isTrainMode || settingsMember == null) return;
          void patchMultiplier
            .mutateAsync({
              trainId: selectedTarget.trainId,
              memberId: settingsMember.memberId,
              speedMultiplier: multiplier,
            })
            .then(() => {
              setSettingsMemberId(null);
            });
        }}
      />
      <ThrottleCockpit
        layoutId={layoutID}
        onOpenSetup={onOpenSetup}
        vehicles={vehicles}
        trains={trains}
        selectedTarget={selectedTarget}
        onSelectTarget={selectTarget}
        witnessLabel={
          isTrainMode ? trainCtx.leadingName ?? undefined : undefined
        }
        speed={cockpitSpeed}
        maxSpeed={maxSpeed}
        forward={forward}
        functions={functions}
        configuredFunctions={configuredFunctions}
        functionPanel={trainAccordion}
        disabled={witnessAddr == null}
        headerExtra={headerExtra}
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
          <AutoDismissAlert severity="warning" resetKey={lastError}>
            {translateErrorCode(
              t as unknown as (
                k: string,
                opts?: { defaultValue?: string },
              ) => string,
              lastError,
              t("throttle:errors.command_station_disconnected"),
            )}
          </AutoDismissAlert>
        )}
      </Box>
    </Box>
  );
}
