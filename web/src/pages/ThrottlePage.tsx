import { useEffect, useMemo, useState } from "react";
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

import { useSocket } from "../context/SocketContext";
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

// ThrottlePage is the throttle UI specified in §6.3b / §7e.7. It
// renders inside the existing AppShell <Outlet/> rather than as a
// full-screen overlay (the M4.5 milestone introduces the page; the
// overlay variant lands later when the AppShell drawer arrives).
export default function ThrottlePage() {
  const { session, setCommandStation, connected } = useSocket();
  const me = useMe().data;
  const { t } = useTranslation(["throttle", "common", "errors"]);

  const layoutID = me?.layoutId ?? null;
  const stations = session?.availableCommandStations ?? [];
  const currentCS = session?.currentSession?.commandStationId ?? 0;
  const activeStation = useMemo(
    () => stations.find((s) => s.id === currentCS),
    [stations, currentCS],
  );

  const [spawnError, setSpawnError] = useState<string | null>(null);

  const handleCSChange = async (csID: number) => {
    setSpawnError(null);
    const result = await setCommandStation(csID);
    if (!result.ok) {
      setSpawnError(result.error ?? "dcc_bus_unavailable");
    }
  };

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
        {activeStation && (
          <DataPlaneStatusChip />
        )}
      </Stack>

      <CommandStationPicker
        stations={stations}
        currentID={currentCS}
        onChange={handleCSChange}
      />

      {spawnError && (
        <Alert severity="error" sx={{ mt: 2 }}>
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

      {currentCS === 0 && (
        <Alert severity="info" sx={{ mt: 2 }}>
          {stations.length === 0
            ? t("throttle:noCommandStations")
            : t("throttle:selectCommandStation")}
        </Alert>
      )}

      {currentCS !== 0 && activeStation?.wsUrl == null && (
        <Alert severity="info" icon={false} sx={{ mt: 2 }}>
          {t("throttle:csStatus.spawning")}
        </Alert>
      )}

      {currentCS !== 0 && activeStation?.wsUrl && layoutID && (
        <DccBusProvider wsUrl={activeStation.wsUrl}>
          <ThrottleSurface layoutID={layoutID} />
        </DccBusProvider>
      )}
    </Container>
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
  onChange,
}: {
  stations: { id: number; name: string; kind: string }[];
  currentID: number;
  onChange: (id: number) => void;
}) {
  const { t } = useTranslation("throttle");
  if (stations.length === 0) return null;
  return (
    <FormControl fullWidth>
      <InputLabel>{t("commandStation")}</InputLabel>
      <Select
        value={currentID > 0 ? String(currentID) : ""}
        label={t("commandStation")}
        onChange={(ev) => onChange(Number(ev.target.value))}
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

function ThrottleSurface({ layoutID }: { layoutID: number }) {
  const roster = useLayoutVehicles(layoutID).data ?? [];
  const drivable = roster.filter((v) => v.dccAddress != null);
  const [selectedAddr, setSelectedAddr] = useState<number | null>(null);
  const bus = useDccBus();
  const { t } = useTranslation(["throttle", "errors"]);

  useEffect(() => {
    if (selectedAddr == null) return;
    void bus.subscribe([selectedAddr]);
  }, [selectedAddr, bus]);

  // When the roster first arrives we pre-select the first drivable
  // vehicle so the user always sees a working slider.
  useEffect(() => {
    if (selectedAddr == null && drivable.length > 0 && drivable[0].dccAddress) {
      setSelectedAddr(drivable[0].dccAddress);
    }
  }, [drivable, selectedAddr]);

  const state =
    selectedAddr != null ? bus.states.get(selectedAddr) : undefined;
  const speed = state?.speed ?? 0;
  const forward = state?.forward ?? true;
  const functions = state?.functions ?? [];

  const handleSpeed = (next: number) => {
    if (selectedAddr == null) return;
    void bus.setSpeed(selectedAddr, next, forward);
  };
  const handleDir = (newDir: "fwd" | "rev") => {
    if (selectedAddr == null) return;
    void bus.setSpeed(selectedAddr, speed, newDir === "fwd");
  };
  const handleFn = (n: number) => {
    if (selectedAddr == null) return;
    void bus.setFunction(selectedAddr, n, !(functions[n] ?? false));
  };
  const handleEStop = () => {
    if (selectedAddr == null) return;
    void bus.setSpeed(selectedAddr, 0, forward, true);
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
            value={speed}
            min={0}
            max={127}
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

        {bus.lastError && (
          <Alert severity="warning">
            {translateErrorCode(
              t as unknown as (
                k: string,
                opts?: { defaultValue?: string },
              ) => string,
              bus.lastError,
              t("throttle:errors.command_station_disconnected"),
            )}
          </Alert>
        )}
      </Stack>
    </Paper>
  );
}
