import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  Container,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Slider,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from "@mui/material";
import EmergencyIcon from "@mui/icons-material/EmergencyShare";
import { useTranslation } from "react-i18next";

import {
  useSocket,
  type CommandStationChangedPayload,
} from "../context/SocketContext";
import { DccBusProvider, useDccBus } from "../context/DccBusContext";
import { useMe } from "../api/auth";
import { useLayoutVehicles } from "../api/vehicles";

// translateErrorCode looks the daemon's machine-readable error up in
// the throttle:errors namespace. The cast goes through `unknown`
// because the i18next typed key union can't model a runtime-derived
// key — we accept the lookup falling through to the generic
// "command station disconnected" message when the code is missing.
function translateErrorCode(
  t: (k: string, opts?: { defaultValue?: string }) => string,
  code: string,
  fallback: string,
): string {
  const key = `throttle:errors.${code}` as unknown as string;
  const resolved = t(key, { defaultValue: "" });
  return resolved && resolved !== key ? resolved : fallback;
}

// maxSpeedValue maps the catalogue speedSteps (14 / 28 / 128) to the
// highest throttle value the dcc-bus wire protocol accepts for that mode.
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

// ThrottlePage is the throttle UI specified in §6.3b / §7e.7. It
// renders inside the existing AppShell <Outlet/> rather than as a
// full-screen overlay (the M4.5 milestone introduces the page; the
// overlay variant lands later when the AppShell drawer arrives).
export default function ThrottlePage() {
  const { session, setCommandStation, connected, reconnecting, subscribe } =
    useSocket();
  const me = useMe().data;
  const { t } = useTranslation(["throttle", "common", "errors"]);

  const layoutID = me?.layoutId ?? null;
  const stations = session?.availableCommandStations ?? [];
  const [selectedCS, setSelectedCS] = useState(0);
  const [selecting, setSelecting] = useState(false);
  const [spawnAcked, setSpawnAcked] = useState(false);
  const [spawnError, setSpawnError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);
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

  // The server may push wsUrl on commandStationChanged before (or
  // without) the setCommandStation ack — honour that so a running
  // dcc-bus is not blocked on a dropped ack frame.
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

  // Restore server-side pick after reconnect (session.opened).
  useEffect(() => {
    const fromSession = session?.currentSession?.commandStationId ?? 0;
    if (fromSession > 0) {
      setSelectedCS(fromSession);
    }
  }, [session?.sessionId]);

  // Single attached station: pre-select so the user does not need a
  // no-op MUI Select click (onChange does not fire when re-picking the
  // same value).
  useEffect(() => {
    if (stations.length === 1 && selectedCS === 0) {
      setSelectedCS(stations[0].id);
    }
  }, [stations, selectedCS]);

  // Always call session.setCommandStation when a cs is selected. Do not
  // skip when session.opened carried a stale wsUrl from a cached port.
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

  // Render order:
  //   1. Header strip with control-plane / data-plane status chips.
  //   2. Command-station picker.
  //   3. The actual throttle area (only when a station is selected).
  return (
    <Container maxWidth="md" sx={{ py: 3 }}>
      <Typography variant="h4" gutterBottom>
        {t("throttle:title")}
      </Typography>
      <Stack direction="row" spacing={1} sx={{ mb: 2 }}>
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
        <Alert severity="warning" sx={{ mb: 2 }}>
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
          sx={{ mt: 2 }}
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
        <Alert severity="info" sx={{ mt: 2 }}>
          {stations.length === 0
            ? t("throttle:noCommandStations")
            : t("throttle:selectCommandStation")}
        </Alert>
      )}

      {selectedCS !== 0 &&
        activeWsUrl == null &&
        !spawnError &&
        (selecting || spawnAcked) && (
        <Alert severity="info" icon={false} sx={{ mt: 2 }}>
          {t("throttle:csStatus.spawning")}
        </Alert>
      )}

      {selectedCS !== 0 && activeWsUrl && layoutID && (
        <DccBusProvider wsUrl={activeWsUrl}>
          <ReconnectingAlert />
          <Stack direction="row" spacing={1} sx={{ mb: 2 }}>
            <DataPlaneStatusChip />
          </Stack>
          <ThrottleSurface
            layoutID={layoutID}
            speedSteps={activeStation?.speedSteps ?? 128}
          />
        </DccBusProvider>
      )}
    </Container>
  );
}

function ReconnectingAlert() {
  const { reconnecting } = useDccBus();
  const { t } = useTranslation("throttle");
  if (!reconnecting) {
    return null;
  }
  return (
    <Alert severity="warning" sx={{ mt: 2 }}>
      {t("reconnecting")}
    </Alert>
  );
}

function DataPlaneStatusChip() {
  const { status } = useDccBus();
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

function ThrottleSurface({
  layoutID,
  speedSteps: sessionSpeedSteps,
}: {
  layoutID: number;
  speedSteps: number;
}) {
  const roster = useLayoutVehicles(layoutID).data ?? [];
  const drivable = roster.filter((v) => v.dccAddress != null);
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

  // Subscribe once per (vehicle, roster, WS open) — not on every
  // loco.state push (the whole context used to change each tick).
  const rosterAddrKey = drivable
    .map((v) => v.dccAddress)
    .filter((a): a is number => a != null)
    .join(",");
  useEffect(() => {
    if (selectedAddr == null || status !== "open") {
      return;
    }
    void subscribe([selectedAddr]);
  }, [selectedAddr, subscribe, rosterAddrKey, status]);

  // When the roster first arrives we pre-select the first drivable
  // vehicle so the user always sees a working slider.
  useEffect(() => {
    if (selectedAddr == null && drivable.length > 0 && drivable[0].dccAddress) {
      setSelectedAddr(drivable[0].dccAddress);
    }
  }, [drivable, selectedAddr]);

  const state =
    selectedAddr != null ? states.get(selectedAddr) : undefined;
  const speed = state?.speed ?? 0;
  const forward = state?.forward ?? true;
  const functions = state?.functions ?? [];

  const handleSpeed = (next: number) => {
    if (selectedAddr == null) return;
    void setSpeed(selectedAddr, next, forward);
  };
  const handleDir = (newDir: "fwd" | "rev") => {
    if (selectedAddr == null) return;
    void setSpeed(selectedAddr, speed, newDir === "fwd");
  };
  const handleFn = (n: number) => {
    if (selectedAddr == null) return;
    void setFunction(selectedAddr, n, !(functions[n] ?? false));
  };
  const handleEStop = () => {
    if (selectedAddr == null) return;
    void setSpeed(selectedAddr, 0, forward, true);
  };

  return (
    <Paper sx={{ mt: 2, p: 3 }} variant="outlined">
      <Stack spacing={3}>
        <FormControl fullWidth>
          <InputLabel>{t("throttle:vehicle")}</InputLabel>
          <Select
            value={selectedAddr != null ? String(selectedAddr) : ""}
            label={t("throttle:vehicle")}
            onChange={(ev) => setSelectedAddr(Number(ev.target.value))}
          >
            {drivable.map((v) => (
              <MenuItem
                key={v.id}
                value={v.dccAddress != null ? String(v.dccAddress) : ""}
              >
                {v.name} ({v.dccAddress})
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <Box>
          <Typography gutterBottom>
            {t("throttle:speed")}: {speed}
          </Typography>
          <Slider
            value={Math.min(speed, maxSpeed)}
            min={0}
            max={maxSpeed}
            step={1}
            onChange={(_, v) => handleSpeed(Array.isArray(v) ? v[0] : v)}
            disabled={selectedAddr == null}
          />
        </Box>

        <ToggleButtonGroup
          exclusive
          value={forward ? "fwd" : "rev"}
          onChange={(_, v) => v && handleDir(v as "fwd" | "rev")}
          disabled={selectedAddr == null}
        >
          <ToggleButton value="fwd">{t("throttle:direction.forward")}</ToggleButton>
          <ToggleButton value="rev">{t("throttle:direction.reverse")}</ToggleButton>
        </ToggleButtonGroup>

        <Box>
          <Typography gutterBottom>{t("throttle:functions")}</Typography>
          <Stack direction="row" flexWrap="wrap" gap={1}>
            {Array.from({ length: 13 }).map((_, n) => {
              const on = Boolean(functions[n]);
              return (
                <ToggleButton
                  key={n}
                  value={n}
                  selected={on}
                  onChange={() => handleFn(n)}
                  size="small"
                  disabled={selectedAddr == null}
                >
                  {t("throttle:fnLabel", { n })}
                </ToggleButton>
              );
            })}
          </Stack>
        </Box>

        <Button
          color="error"
          variant="contained"
          startIcon={<EmergencyIcon />}
          onClick={handleEStop}
          disabled={selectedAddr == null}
          size="large"
        >
          {t("throttle:emergencyStop")}
        </Button>

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
      </Stack>
    </Paper>
  );
}
