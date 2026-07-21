import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  FormControlLabel,
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
import { useLayoutSupervisordSync } from "../api/presence";
import { useReceivedLeases } from "../api/leases";
import { useVehicleFunctions } from "../api/functions";
import {
  useLayoutTrains,
  useLayoutVehicles,
  usePatchTrainMemberSettings,
  type RosterTrain,
} from "../api/vehicles";
import type { ThrottleCockpitFunction } from "../components/throttle/ThrottleCockpit";
import AutoDismissAlert from "../components/AutoDismissAlert";
import ThrottleCockpit from "../components/throttle/ThrottleCockpit";
import TrainFunctionAccordions, {
  type TrainAccordionMember,
} from "../components/throttle/TrainFunctionAccordions";
import TrainMemberSettingsDialog from "../components/throttle/TrainMemberSettingsDialog";
import ThrottleNavigationGuard from "../components/throttle/ThrottleNavigationGuard";
import SlotInUseDialog from "../components/throttle/SlotInUseDialog";
import ThrottleSetupDialog from "../components/throttle/ThrottleSetupDialog";
import ThrottleGamepadDialog from "../components/throttle/ThrottleGamepadDialog";
import CommandStationPicker from "../components/throttle/CommandStationPicker";
import { useDebouncedSpeedSend } from "../hooks/useDebouncedSpeedSend";
import { useGamepads } from "../hooks/useGamepads";
import { useGamepadControl } from "../hooks/useGamepadControl";
import {
  defaultGamepadMapping,
  loadGamepadMapping,
  saveGamepadMapping,
  type GamepadMapping,
} from "../hooks/gamepadMapping";
import { useRadioStopSound } from "../hooks/useRadioStopSound";
import { useThrottleSpeedOverride } from "../hooks/useThrottleSpeedOverride";
import { useThrottleCommandStationSelection } from "../hooks/useThrottleCommandStationSelection";
import { useDebouncedTrainSpeedSend } from "../hooks/useDebouncedTrainSpeedSend";
import { useKeyedRetryingSend } from "../hooks/useRetryingSend";
import {
  useThrottleTargetSelection,
  type ThrottleTarget,
} from "../hooks/useThrottleTargetSelection";
import { useThrottleFunctionsListView } from "../hooks/useThrottleFunctionsListView";
import { useTrainAccordionExpanded } from "../hooks/useTrainAccordionExpanded";
import { useDriverRadioInbound } from "../hooks/useDriverRadioInbound";
import { buildThrottleRadioHeader } from "../hooks/useThrottleRadioChat";
import type { DriverRadioInbound } from "../hooks/useDriverRadioInbound";
import {
  TakeoverDriverDialog,
  useTakeoverDriverSession,
} from "../components/throttle/TakeoverDriverDialog";
import LeaseCountdown from "../components/leases/LeaseCountdown";
import { useLeaseEvents } from "../hooks/useLeaseEvents";
import {
  effectiveLeaseMaxSpeed,
  useThrottleLease,
} from "../hooks/useThrottleLease";

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
  const {
    session,
    setCommandStation,
    connected,
    reconnecting,
    subscribe,
    refreshSession,
  } = useSocket();
  const me = useMe().data;
  const radioChatEnabled = me?.radioChatEnabled ?? true;
  const { t } = useTranslation(["throttle", "common", "errors"]);

  const layoutID = me?.layoutId ?? null;
  useLayoutSupervisordSync(layoutID);
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
  const { functionsAsList, setFunctionsAsList } = useThrottleFunctionsListView();

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

      <CommandStationPicker
        stations={stations}
        currentID={selectedCS}
        disabled={selecting}
        allowClear
        onChange={handlePickerChange}
        onRefresh={refreshSession}
        refreshDisabled={!connected || reconnecting}
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

      {selectedCS === 0 && stations.length > 0 && (
        <AutoDismissAlert severity="info" resetKey="select-cs">
          {t("throttle:selectCommandStation")}
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
      <FormControlLabel
        control={
          <Checkbox
            checked={functionsAsList}
            onChange={(ev) => setFunctionsAsList(ev.target.checked)}
          />
        }
        label={t("throttle:setup.functionsAsList")}
      />
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
      {radioChatEnabled && driverRadio.alertNode}
      {radioChatEnabled && driverRadio.overlay}
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
              radioChatEnabled={radioChatEnabled}
              functionsAsList={functionsAsList}
            />
          </DccBusProvider>
        ) : (
          <>
            {setupDialog}
            <IdleThrottle
              layoutID={layoutID}
              onOpenSetup={openSetup}
              driverRadio={driverRadio}
              radioChatEnabled={radioChatEnabled}
              functionsAsList={functionsAsList}
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
    </>
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
  vehicles: { id: string; dccAddress: number }[],
  selectedAddr: number | null,
): ThrottleCockpitFunction[] {
  const vehicleId =
    selectedAddr != null
      ? vehicles.find((v) => v.dccAddress === selectedAddr)?.id
      : undefined;
  const fnList = useVehicleFunctions(vehicleId ?? "").data ?? [];
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
): { vehicleId: string | null; vehicleName: string | null } {
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

type TrainMemberContext = TrainAccordionMember & {
  excludeFromSpeed: boolean;
  startDelayMs: number;
  accelRampMs: number;
  accelRampMaxSteps: number;
  brakeRampMs: number;
  brakeRampMaxSteps: number;
};

function trainMemberDccAddresses(members: TrainMemberContext[]): number[] {
  return members.flatMap((m) => (m.dccAddress != null ? [m.dccAddress] : []));
}

function findLeadingMember(train: RosterTrain, vehiclesById: Map<string, { name: string; dccAddress: number | null }>) {
  const sorted = [...train.members].sort((a, b) => a.position - b.position);
  for (const m of sorted) {
    if (m.excludeFromSpeed) continue;
    const v = vehiclesById.get(m.vehicleId);
    if (v?.dccAddress != null) {
      return { member: m, vehicle: v, dccAddress: v.dccAddress };
    }
  }
  return null;
}

function useSelectedTrainContext(layoutID: number, trainId: string | null) {
  const trains = useLayoutTrains(layoutID).data ?? [];
  const vehicles = useLayoutVehicles(layoutID).data ?? [];
  return useMemo(() => {
    if (trainId == null) {
      return {
        train: null as RosterTrain | null,
        leadingAddr: null as number | null,
        leadingName: null as string | null,
        members: [] as TrainMemberContext[],
      };
    }
    const train = trains.find((tr) => tr.id === trainId) ?? null;
    if (!train) {
      return {
        train: null,
        leadingAddr: null,
        leadingName: null,
        members: [],
      };
    }
    const byId = new Map(
      vehicles.map((v) => [v.id, { name: v.name, dccAddress: v.dccAddress }]),
    );
    const leading = findLeadingMember(train, byId);
    const sorted = [...train.members].sort((a, b) => a.position - b.position);
    const members = sorted.flatMap((m) => {
      const v = byId.get(m.vehicleId);
      if (!v) return [];
      const isDummy = v.dccAddress == null;
      const mult = m.speedMultiplier > 0 ? m.speedMultiplier : 1;
      return [
        {
          memberId: m.id,
          vehicleId: m.vehicleId,
          name: v.name,
          dccAddress: v.dccAddress,
          isDummy,
          isLeading: !isDummy && leading?.member.id === m.id,
          speedMultiplier: mult,
          excludeFromSpeed: m.excludeFromSpeed,
          startDelayMs: m.startDelayMs ?? 0,
          accelRampMs: m.accelRampMs ?? 0,
          accelRampMaxSteps: m.accelRampMaxSteps > 0 ? m.accelRampMaxSteps : 1,
          brakeRampMs: m.brakeRampMs ?? 0,
          brakeRampMaxSteps: m.brakeRampMaxSteps > 0 ? m.brakeRampMaxSteps : 1,
        },
      ];
    });
    return {
      train,
      leadingAddr: leading?.dccAddress ?? null,
      leadingName: leading?.vehicle.name ?? null,
      members,
    };
  }, [trainId, trains, vehicles]);
}


function IdleThrottle({
  layoutID,
  onOpenSetup,
  driverRadio,
  radioChatEnabled,
  functionsAsList,
}: {
  layoutID: number;
  onOpenSetup: () => void;
  driverRadio: DriverRadioInbound;
  radioChatEnabled: boolean;
  functionsAsList: boolean;
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
    radioChatEnabled,
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
        functionsAsList={functionsAsList}
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
  radioChatEnabled,
  functionsAsList,
}: {
  layoutID: number;
  speedSteps: number;
  onOpenSetup: () => void;
  driverRadio: DriverRadioInbound;
  radioChatEnabled: boolean;
  functionsAsList: boolean;
}) {
  const me = useMe().data;
  useLeaseEvents();
  const receivedLeases = useReceivedLeases();
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
    isTrainMode ? trainCtx.leadingAddr : selectedAddr,
  );
  const {
    subscribe,
    select,
    selectTrain,
    status,
    states,
    lastError,
    clearLastError,
    setSpeed,
    setTrainSpeed,
    setFunction,
    stealSlot,
    speedSteps: busSpeedSteps,
    reconnecting: dccReconnecting,
  } = useDccBus();
  const { t } = useTranslation(["throttle", "errors"]);
  const speedSteps = busSpeedSteps ?? sessionSpeedSteps;
  const maxSpeed = maxSpeedValue(speedSteps);
  const activeLease = useThrottleLease(
    receivedLeases.data,
    selectedTarget,
    vehicles,
  );
  const throttleMaxSpeed = effectiveLeaseMaxSpeed(
    maxSpeed,
    activeLease?.speedLimit,
  );
  // Only spin once a connection was established and then lost — not during
  // the very first connect, which would flash the icon on every page load.
  const connectionLost =
    dccReconnecting || status === "closed" || status === "error";

  const subscribeAddrs = useMemo(() => {
    if (isTrainMode) {
      return trainMemberDccAddresses(trainCtx.members);
    }
    if (selectedAddr != null) return [selectedAddr];
    return [];
  }, [isTrainMode, trainCtx.members, selectedAddr]);

  const rosterAddrKey = subscribeAddrs.join(",");
  useEffect(() => {
    if (subscribeAddrs.length === 0 || status !== "open") return;
    void subscribe(subscribeAddrs);
  }, [subscribeAddrs, subscribe, rosterAddrKey, status]);

  // Reserve the command-station slot for the active drive target on switch
  // (driver-only model). The backend defers the previous slot's release by
  // the switcher grace window so A→B→A reuses A's slot. SetSpeed also
  // reserves, but sending select up front guarantees the slot before the
  // first throttle move and lets diag show the lease immediately.
  useEffect(() => {
    if (status !== "open") return;
    if (isTrainMode) {
      if (selectedTarget?.kind === "train") {
        void selectTrain(selectedTarget.trainId);
      }
      return;
    }
    if (selectedAddr == null) return;
    void select(selectedAddr);
  }, [select, selectTrain, status, selectedAddr, isTrainMode, selectedTarget]);

  // Picking a loco in the select box re-fetches its live state from the
  // dcc-bus websocket: the daemon answers loco.subscribe with the current
  // loco.state (speed, direction, functions), which is what drives the slider
  // and the function buttons. We subscribe here explicitly — not only via the
  // subscribeAddrs effect above — so re-picking the same loco also refreshes a
  // possibly stale view. Trains rely on the effect, which re-runs once their
  // powered members resolve.
  const handleSelectTarget = useCallback(
    (target: ThrottleTarget) => {
      selectTarget(target);
      if (target.kind === "vehicle" && status === "open") {
        void subscribe([target.dccAddress]);
      }
    },
    [selectTarget, subscribe, status],
  );

  const witnessAddr = isTrainMode ? trainCtx.leadingAddr : selectedAddr;
  const state = witnessAddr != null ? states.get(witnessAddr) : undefined;
  const serverSpeed = state?.speed ?? 0;
  const forward = state?.forward ?? true;
  const functions = state?.functions ?? [];

  const { displaySpeed, noteUserSpeed } = useThrottleSpeedOverride(
    serverSpeed,
    witnessAddr,
  );
  const cockpitSpeed = Math.min(displaySpeed, throttleMaxSpeed);
  const { queueSpeed, sendSpeedNow, flush, retrying: speedRetrying } =
    useDebouncedSpeedSend(setSpeed);
  const {
    queueSpeed: queueTrainSpeed,
    sendSpeedNow: sendTrainSpeedNow,
    flush: flushTrain,
    retrying: trainSpeedRetrying,
  } = useDebouncedTrainSpeedSend(setTrainSpeed);
  const { dispatch: sendFunction, retrying: functionRetrying } =
    useKeyedRetryingSend(
    setFunction,
    (address: number, fn: number) => `${address}:${fn}`,
  );
  const commandRetrying =
    speedRetrying || trainSpeedRetrying || functionRetrying;
  const isMoving = cockpitSpeed > 0 && witnessAddr != null;

  const [settingsMemberId, setSettingsMemberId] = useState<number | null>(null);
  const [gamepadOpen, setGamepadOpen] = useState(false);
  const [gamepadMapping, setGamepadMapping] = useState<GamepadMapping | null>(
    null,
  );
  const { gamepads } = useGamepads();
  const patchMemberSettings = usePatchTrainMemberSettings();
  const { expandedMemberIds, toggleMember } = useTrainAccordionExpanded(
    isTrainMode ? selectedTarget.trainId : null,
  );

  useEffect(() => {
    if (gamepads.length === 0) {
      return;
    }
    const pad = gamepads[0];
    setGamepadMapping((prev) => {
      if (prev?.gamepadId === pad.id) {
        return prev;
      }
      return loadGamepadMapping(pad.id);
    });
  }, [gamepads]);

  const activeGamepadIndex =
    gamepads.find((gp) => gp.id === gamepadMapping?.gamepadId)?.index ??
    gamepads[0]?.index ??
    null;

  // Keep volatile drive context in refs so speed/fn handlers stay referentially
  // stable across pointermove-driven re-renders (VerticalThrottle / gamepad).
  const driveCtxRef = useRef({
    isTrainMode,
    selectedTarget,
    selectedAddr,
    witnessAddr,
    throttleMaxSpeed,
    forward,
    cockpitSpeed,
    functions,
    trainMembers: trainCtx.members,
    states,
  });
  driveCtxRef.current = {
    isTrainMode,
    selectedTarget,
    selectedAddr,
    witnessAddr,
    throttleMaxSpeed,
    forward,
    cockpitSpeed,
    functions,
    trainMembers: trainCtx.members,
    states,
  };

  const handleSpeed = useCallback(
    (next: number) => {
      const ctx = driveCtxRef.current;
      const clamped = Math.min(next, ctx.throttleMaxSpeed);
      if (ctx.isTrainMode && ctx.selectedTarget?.kind === "train") {
        noteUserSpeed(clamped);
        queueTrainSpeed(ctx.selectedTarget.trainId, clamped, ctx.forward);
        return;
      }
      if (ctx.selectedAddr == null) return;
      noteUserSpeed(clamped);
      queueSpeed(ctx.selectedAddr, clamped, ctx.forward);
    },
    [noteUserSpeed, queueSpeed, queueTrainSpeed],
  );

  const handleDir = useCallback(
    (fwd: boolean) => {
      const ctx = driveCtxRef.current;
      if (ctx.isTrainMode && ctx.selectedTarget?.kind === "train") {
        sendTrainSpeedNow(
          ctx.selectedTarget.trainId,
          ctx.cockpitSpeed,
          fwd,
        );
        return;
      }
      if (ctx.selectedAddr == null) return;
      sendSpeedNow(ctx.selectedAddr, ctx.cockpitSpeed, fwd);
    },
    [sendSpeedNow, sendTrainSpeedNow],
  );

  const handleFn = useCallback(
    (n: number) => {
      const ctx = driveCtxRef.current;
      const fnAddr = ctx.isTrainMode ? ctx.witnessAddr : ctx.selectedAddr;
      if (fnAddr == null) return;
      sendFunction(fnAddr, n, !(ctx.functions[n] ?? false));
    },
    [sendFunction],
  );

  const handleTrainFn = useCallback(
    (memberId: number, fn: number) => {
      const ctx = driveCtxRef.current;
      const member = ctx.trainMembers.find((m) => m.memberId === memberId);
      if (!member || member.dccAddress == null) return;
      const memberState = ctx.states.get(member.dccAddress);
      const memberFns = memberState?.functions ?? [];
      sendFunction(member.dccAddress, fn, !(memberFns[fn] ?? false));
    },
    [sendFunction],
  );

  const handleStop = useCallback(() => {
    const ctx = driveCtxRef.current;
    noteUserSpeed(0);
    if (ctx.isTrainMode && ctx.selectedTarget?.kind === "train") {
      sendTrainSpeedNow(ctx.selectedTarget.trainId, 0, ctx.forward);
      return;
    }
    if (ctx.selectedAddr == null) return;
    sendSpeedNow(ctx.selectedAddr, 0, ctx.forward);
  }, [noteUserSpeed, sendSpeedNow, sendTrainSpeedNow]);

  const handleAxisEnabledToggle = useCallback(() => {
    setGamepadMapping((prev) => {
      if (prev == null) return prev;
      const next = { ...prev, axisEnabled: prev.axisEnabled === false };
      saveGamepadMapping(next);
      return next;
    });
  }, []);

  const gamepadDisabled =
    gamepadOpen ||
    (isTrainMode
      ? trainMemberDccAddresses(trainCtx.members).length === 0
      : witnessAddr == null);

  useGamepadControl({
    mapping: gamepadMapping,
    gamepadIndex: activeGamepadIndex,
    maxSpeed: throttleMaxSpeed,
    currentSpeed: cockpitSpeed,
    forward,
    disabled: gamepadDisabled,
    onSpeed: handleSpeed,
    onDirectionChange: handleDir,
    onFunctionToggle: handleFn,
    onStop: handleStop,
    onAxisEnabledToggle: handleAxisEnabledToggle,
  });

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
  const radioHeader = buildThrottleRadioHeader({
    layoutId: layoutID,
    vehicleId: drive.vehicleId,
    vehicleName: drive.vehicleName ?? trainCtx.leadingName,
    radio: driverRadio,
    radioChatEnabled,
  });
  const headerExtra = (
    <Stack direction="row" spacing={0.5} alignItems="center">
      {radioHeader}
      {activeLease && (
        <Chip
          size="small"
          label={
            <LeaseCountdown
              expiresAt={activeLease.expiresAt}
              component="span"
              sx={{ color: "inherit" }}
            />
          }
          sx={{ color: "inherit", borderColor: "rgba(255,255,255,0.35)" }}
          variant="outlined"
        />
      )}
    </Stack>
  );

  const settingsMember =
    settingsMemberId != null
      ? trainCtx.members.find((m) => m.memberId === settingsMemberId)
      : undefined;

  const trainAccordion = isTrainMode ? (
    <TrainFunctionAccordions
      members={trainCtx.members}
      states={states}
      expandedMemberIds={expandedMemberIds}
      onToggleExpanded={toggleMember}
      onFunctionToggle={handleTrainFn}
      onOpenSettings={setSettingsMemberId}
      showMultiplierCog={trainCtx.train?.ownerId === me?.id}
      functionsAsList={functionsAsList}
      disabled={trainMemberDccAddresses(trainCtx.members).length === 0}
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
      <SlotInUseDialog
        open={lastError?.code === "slot_in_use"}
        address={
          lastError?.address && lastError.address > 0
            ? lastError.address
            : (selectedAddr ?? 0)
        }
        onDismiss={clearLastError}
        onSteal={stealSlot}
      />
      <TrainMemberSettingsDialog
        open={settingsMember != null}
        memberName={settingsMember?.name ?? ""}
        isLeading={settingsMember?.isLeading ?? false}
        initialSettings={{
          speedMultiplier: settingsMember?.speedMultiplier ?? 1,
          excludeFromSpeed: settingsMember?.excludeFromSpeed ?? false,
          startDelayMs: settingsMember?.startDelayMs ?? 0,
          accelRampMs: settingsMember?.accelRampMs ?? 0,
          accelRampMaxSteps: settingsMember?.accelRampMaxSteps ?? 1,
          brakeRampMs: settingsMember?.brakeRampMs ?? 0,
          brakeRampMaxSteps: settingsMember?.brakeRampMaxSteps ?? 1,
        }}
        saving={patchMemberSettings.isPending}
        onClose={() => setSettingsMemberId(null)}
        onSave={(settings) => {
          if (!isTrainMode || settingsMember == null || settingsMember.isDummy) return;
          const isLeadingMember = settingsMember.isLeading;
          void patchMemberSettings
            .mutateAsync({
              trainId: selectedTarget.trainId,
              memberId: settingsMember.memberId,
              ...(isLeadingMember
                ? {}
                : {
                    speedMultiplier: settings.speedMultiplier,
                    excludeFromSpeed: settings.excludeFromSpeed,
                  }),
              startDelayMs: settings.startDelayMs,
              accelRampMs: settings.accelRampMs,
              accelRampMaxSteps: settings.accelRampMaxSteps,
              brakeRampMs: settings.brakeRampMs,
              brakeRampMaxSteps: settings.brakeRampMaxSteps,
            })
            .then(() => {
              setSettingsMemberId(null);
            });
        }}
      />
      {gamepadOpen && (
        <ThrottleGamepadDialog
          open={gamepadOpen}
          onClose={() => setGamepadOpen(false)}
          gamepads={gamepads}
          configuredFunctions={configuredFunctions}
          mapping={
            gamepadMapping ??
            defaultGamepadMapping(gamepads[0]?.id ?? "default")
          }
          maxSpeed={throttleMaxSpeed}
          onMappingChange={setGamepadMapping}
          onConfirm={(next) => {
            setGamepadMapping(next);
            saveGamepadMapping(next);
          }}
        />
      )}
      <ThrottleCockpit
        layoutId={layoutID}
        onOpenSetup={onOpenSetup}
        onOpenGamepad={() => setGamepadOpen(true)}
        gamepadActive={gamepadMapping?.enabled === true}
        vehicles={vehicles}
        trains={trains}
        selectedTarget={selectedTarget}
        onSelectTarget={handleSelectTarget}
        speed={cockpitSpeed}
        maxSpeed={throttleMaxSpeed}
        forward={forward}
        functions={functions}
        configuredFunctions={configuredFunctions}
        functionPanel={trainAccordion}
        functionsAsList={functionsAsList}
        disabled={witnessAddr == null}
        connectionLost={connectionLost}
        commandRetrying={commandRetrying}
        headerExtra={headerExtra}
        onSpeedChange={handleSpeed}
        onDirectionChange={handleDir}
        onFunctionToggle={handleFn}
        onStop={handleStop}
      />

      {lastError && lastError.code !== "slot_in_use" && (
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
          <AutoDismissAlert severity="warning" resetKey={lastError.code}>
            {translateErrorCode(
              t as unknown as (
                k: string,
                opts?: { defaultValue?: string },
              ) => string,
              lastError.code,
              t("throttle:errors.command_station_disconnected"),
            )}
          </AutoDismissAlert>
        </Box>
      )}
    </Box>
  );
}
