import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  LinearProgress,
  Stack,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useMe } from "../../api/auth";
import { useVehicleFunctions } from "../../api/functions";
import {
  useTakeoverActions,
  type TakeoverGrantedEvent,
  type TakeoverRequestedEvent,
} from "../../api/takeover";
import { useLayoutTrains, useLayoutVehicles } from "../../api/vehicles";
import { DccBusProvider, useDccBus } from "../../context/DccBusContext";
import { useSocket } from "../../context/SocketContext";
import ThrottleCockpit from "../throttle/ThrottleCockpit";
import { useDebouncedSpeedSend } from "../../hooks/useDebouncedSpeedSend";
import { useKeyedRetryingSend } from "../../hooks/useRetryingSend";
import { useThrottleSpeedOverride } from "../../hooks/useThrottleSpeedOverride";
import { useThrottleFunctionsListView } from "../../hooks/useThrottleFunctionsListView";
import { useThrottleHardwareKeys } from "../../hooks/useThrottleHardwareKeys";

interface TakeoverThrottleOverlayProps {
  layoutId: number;
  grant: TakeoverGrantedEvent;
  wsUrl: string;
  onClose: () => void;
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

// TakeoverThrottleOverlay hosts a throttle surface for a granted
// takeover; release is allowed only at speed 0 and revokes the lease.
export default function TakeoverThrottleOverlay({
  layoutId,
  grant,
  wsUrl,
  onClose,
}: TakeoverThrottleOverlayProps) {
  return (
    <DccBusProvider wsUrl={wsUrl}>
      <TakeoverOverlayBody layoutId={layoutId} grant={grant} onClose={onClose} />
    </DccBusProvider>
  );
}

function TakeoverOverlayBody({
  layoutId,
  grant,
  onClose,
}: {
  layoutId: number;
  grant: TakeoverGrantedEvent;
  onClose: () => void;
}) {
  const { t } = useTranslation(["interlocking", "throttle"]);
  const { releaseTakeover } = useTakeoverActions();
  const vehicles = useLayoutVehicles(layoutId).data ?? [];
  const trains = useLayoutTrains(layoutId).data ?? [];
  const dcc = useDccBus();
  const { subscribe, status, states, speedSteps: busSpeedSteps, setSpeed, setFunction } = dcc;

  const drive = useMemo(() => {
    if (grant.target === "vehicle") {
      const v = vehicles.find((row) => row.id === grant.targetId);
      if (!v || v.dccAddress == null) {
        return null;
      }
      return {
        id: v.id,
        name: v.name,
        dccAddress: v.dccAddress,
        addresses: [v.dccAddress],
      };
    }
    const tr = trains.find((row) => row.id === grant.targetId);
    if (!tr) {
      return null;
    }
    const addrs: number[] = [];
    for (const m of tr.members) {
      const v = vehicles.find((row) => row.id === m.vehicleId);
      if (v?.dccAddress != null) {
        addrs.push(v.dccAddress);
      }
    }
    const lead = addrs[0];
    if (lead == null) {
      return null;
    }
    return {
      id: tr.id,
      name: tr.name,
      dccAddress: lead,
      addresses: addrs,
    };
  }, [grant, vehicles, trains]);

  useEffect(() => {
    if (!drive || status !== "open") {
      return;
    }
    void subscribe(drive.addresses);
  }, [subscribe, status, drive]);

  const state = drive != null ? states.get(drive.dccAddress) : undefined;
  const speed = state?.speed ?? 0;
  const forward = state?.forward ?? true;
  const functions = state?.functions ?? [];
  const maxSpeed = maxSpeedValue(busSpeedSteps ?? 128);
  const canClose = speed === 0;
  const { functionsAsList } = useThrottleFunctionsListView();

  const fnList = useVehicleFunctions(
    grant.target === "vehicle" ? grant.targetId : "",
  ).data ?? [];
  const configuredFunctions = useMemo(
    () =>
      [...fnList]
        .sort((a, b) => a.position - b.position)
        .map((f) => ({ num: f.num, label: f.name, icon: f.icon })),
    [fnList],
  );

  const { displaySpeed, noteUserSpeed } = useThrottleSpeedOverride(
    speed,
    drive?.dccAddress ?? null,
  );
  const cockpitSpeed = Math.min(displaySpeed, maxSpeed);
  const { queueSpeed, sendSpeedNow, retrying: speedRetrying } =
    useDebouncedSpeedSend(setSpeed);
  const { dispatch: sendFunction, retrying: functionRetrying } =
    useKeyedRetryingSend(
    setFunction,
    (address: number, fn: number) => `${address}:${fn}`,
  );
  const commandRetrying = speedRetrying || functionRetrying;

  const leaseRemaining = useLeaseCountdown(grant.leaseExpiresAt);
  const [releasing, setReleasing] = useState(false);

  const releaseLabel =
    grant.target === "train"
      ? t("interlocking:view.takeover.releaseTrain")
      : t("interlocking:view.takeover.releaseVehicle");

  const handleRelease = async () => {
    if (releasing) {
      return;
    }
    if (drive && !canClose) {
      return;
    }
    setReleasing(true);
    try {
      await releaseTakeover(grant.requestId);
    } finally {
      onClose();
    }
  };

  const handleTakeoverSpeed = useCallback(
    (next: number) => {
      if (!drive) return;
      noteUserSpeed(next);
      queueSpeed(drive.dccAddress, next, forward);
    },
    [drive, noteUserSpeed, queueSpeed, forward],
  );

  useThrottleHardwareKeys({
    maxSpeed,
    currentSpeed: cockpitSpeed,
    onSpeed: handleTakeoverSpeed,
    disabled: drive == null,
  });

  const handleDialogClose = (
    _: object,
    reason: "backdropClick" | "escapeKeyDown",
  ) => {
    if (reason === "backdropClick" || reason === "escapeKeyDown") {
      void handleRelease();
    }
  };

  if (!drive) {
    return (
      <Dialog open fullWidth maxWidth="md" onClose={() => void handleRelease()}>
        <DialogTitle>{t("interlocking:view.takeover.overlayTitle")}</DialogTitle>
        <DialogContent>
          <Typography color="text.secondary">
            {t("interlocking:view.takeover.noDccTarget")}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => void handleRelease()} disabled={releasing}>
            {releaseLabel}
          </Button>
        </DialogActions>
      </Dialog>
    );
  }

  return (
    <Dialog open fullWidth maxWidth="md" onClose={handleDialogClose}>
      <DialogTitle>
        <Stack
          direction="row"
          spacing={1}
          alignItems="center"
          justifyContent="space-between"
          flexWrap="wrap"
          useFlexGap
        >
          <Typography variant="h6" component="span">
            {t("interlocking:view.takeover.overlayTitle")} — {drive.name}
          </Typography>
          <Stack direction="row" spacing={1} alignItems="center">
            <Chip
              size="small"
              color="warning"
              label={t("interlocking:view.takeover.leaseRemaining", {
                time: leaseRemaining,
              })}
            />
            <Button
              variant="contained"
              color="warning"
              size="small"
              onClick={() => void handleRelease()}
              disabled={!canClose || releasing}
            >
              {releaseLabel}
            </Button>
          </Stack>
        </Stack>
      </DialogTitle>
      <DialogContent sx={{ p: 0, height: "min(80vh, 640px)" }}>
        <Box sx={{ height: "100%", display: "flex", flexDirection: "column" }}>
          <ThrottleCockpit
            layoutId={layoutId}
            onOpenSetup={() => {}}
            vehicles={[{ id: drive.id, name: drive.name, dccAddress: drive.dccAddress }]}
            selectedAddress={drive.dccAddress}
            onSelectAddress={() => {}}
            speed={cockpitSpeed}
            maxSpeed={maxSpeed}
            forward={forward}
            functions={functions}
            configuredFunctions={configuredFunctions}
            functionsAsList={functionsAsList}
            onSpeedChange={handleTakeoverSpeed}
            onDirectionChange={(fwd) => {
              sendSpeedNow(drive.dccAddress, cockpitSpeed, fwd);
            }}
            onFunctionToggle={(n) => {
              sendFunction(drive.dccAddress, n, !(functions[n] ?? false));
            }}
            onStop={() => {
              noteUserSpeed(0);
              sendSpeedNow(drive.dccAddress, 0, forward);
            }}
            commandRetrying={commandRetrying}
          />
        </Box>
      </DialogContent>
    </Dialog>
  );
}

function useLeaseCountdown(expiresAtMs: number): string {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);
  const sec = Math.max(0, Math.ceil((expiresAtMs - now) / 1000));
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

export function useTakeoverSignalmanSession(layoutId: number) {
  const me = useMe().data;
  const { subscribe } = useSocket();
  const [grant, setGrant] = useState<TakeoverGrantedEvent | null>(null);
  const [pending, setPending] = useState<TakeoverRequestedEvent | null>(null);
  const [rejected, setRejected] = useState(false);

  useEffect(() => {
    return subscribe("takeover.requested", (payload) => {
      const data = payload as TakeoverRequestedEvent;
      if (me && data.signalman.userId !== me.id) {
        return;
      }
      setRejected(false);
      setPending(data);
    });
  }, [subscribe, me]);

  useEffect(() => {
    return subscribe("takeover.granted", (payload) => {
      const data = payload as TakeoverGrantedEvent;
      if (me && data.signalman.userId !== me.id) {
        return;
      }
      setPending(null);
      setGrant(data);
    });
  }, [subscribe, me]);

  useEffect(() => {
    return subscribe("takeover.rejected", () => {
      setPending(null);
      setRejected(true);
    });
  }, [subscribe]);

  useEffect(() => {
    return subscribe("takeover.released", (payload) => {
      const data = payload as { requestId?: number };
      setGrant((prev) =>
        prev != null && data.requestId === prev.requestId ? null : prev,
      );
    });
  }, [subscribe]);

  const clearGrant = useCallback(() => setGrant(null), []);
  const clearPending = useCallback(() => setPending(null), []);
  const clearRejected = useCallback(() => setRejected(false), []);

  return {
    grant,
    clearGrant,
    pending,
    clearPending,
    rejected,
    clearRejected,
    layoutId,
  };
}

// TakeoverWaitingDialog shows the signalman a "waiting for takeover"
// countdown while the driver's 15 s reject window runs, with a cancel
// action. It closes when the takeover is granted or rejected.
export function TakeoverWaitingDialog({
  pending,
  onCancel,
}: {
  pending: TakeoverRequestedEvent;
  onCancel: () => void;
}) {
  const { t } = useTranslation(["interlocking", "throttle"]);
  const { cancelTakeover } = useTakeoverActions();
  const [busy, setBusy] = useState(false);
  const remaining = useWaitCountdown(pending.autoGrantAt);

  return (
    <Dialog open onClose={() => {}} maxWidth="xs" fullWidth>
      <DialogTitle>{t("interlocking:view.takeover.waitingTitle")}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary">
          {t("interlocking:view.takeover.waitingMessage", { seconds: remaining })}
        </Typography>
        <LinearProgress sx={{ mt: 2 }} />
      </DialogContent>
      <DialogActions>
        <Button
          color="inherit"
          disabled={busy}
          onClick={() => {
            setBusy(true);
            void cancelTakeover(pending.requestId).finally(() => {
              setBusy(false);
              onCancel();
            });
          }}
        >
          {t("interlocking:view.takeover.waitingCancel")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function useWaitCountdown(autoGrantAtMs: number): number {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(id);
  }, []);
  return Math.max(0, Math.ceil((autoGrantAtMs - now) / 1000));
}
