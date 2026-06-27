import { useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from "@mui/material";
import Z21Icon from "../../components/icons/Z21Icon";
import { Link as RouterLink } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ApiError } from "../../api/client";
import { useMe } from "../../api/auth";
import { useLayoutCommandStations } from "../../api/command_stations";
import { useLayoutVehicles, type RosterVehicle } from "../../api/vehicles";
import {
  useCancelZ21Pairing,
  useStartZ21Pairing,
  useUnpairZ21Remote,
  useUpdateZ21RemoteSession,
  useZ21RemoteClients,
  useZ21RemoteStatus,
  Z21_HANDSET_BRAKE_SECS_DEFAULT,
  Z21_HANDSET_BRAKE_SECS_MAX,
  Z21_HANDSET_BRAKE_SECS_MIN,
  type Z21RemoteClient,
} from "../../api/remotes";

function formatTime(ms: number | undefined, locale: string): string {
  if (ms == null || ms <= 0) return "—";
  return new Date(ms).toLocaleString(locale);
}

function formatDuration(ms: number): string {
  if (ms < 0) ms = 0;
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) {
    return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  }
  return `${m}:${String(s).padStart(2, "0")}`;
}

function useNowTick(enabled: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!enabled) return;
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [enabled]);
  return now;
}

function usePairingCountdown(expiresAt: number | undefined): number | null {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (expiresAt == null) return;
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [expiresAt]);
  if (expiresAt == null) return null;
  return Math.max(0, Math.floor((expiresAt - now) / 1000));
}

export default function Z21RemotePage() {
  const { t, i18n } = useTranslation(["z21Remote", "common", "errors"]);
  const me = useMe().data;
  const layoutId = me?.layoutId ?? null;

  const stations = useLayoutCommandStations(layoutId);
  const z21Stations = useMemo(
    () => (stations.data ?? []).filter((s) => s.z21ServerEnabled),
    [stations.data],
  );

  const [csId, setCsId] = useState<number | "">("");
  useEffect(() => {
    if (z21Stations.length === 0) {
      setCsId("");
      return;
    }
    if (csId === "" || !z21Stations.some((s) => s.id === csId)) {
      setCsId(z21Stations[0].id);
    }
  }, [z21Stations, csId]);

  const selectedCsId = typeof csId === "number" ? csId : null;
  const status = useZ21RemoteStatus(layoutId, selectedCsId);
  const clients = useZ21RemoteClients(layoutId, selectedCsId);
  const now = useNowTick((clients.data?.clients.length ?? 0) > 0);
  const roster = useLayoutVehicles(layoutId);

  const startPairing = useStartZ21Pairing(layoutId ?? 0, selectedCsId ?? 0);
  const cancelPairing = useCancelZ21Pairing(layoutId ?? 0, selectedCsId ?? 0);
  const updateSession = useUpdateZ21RemoteSession(
    layoutId ?? 0,
    selectedCsId ?? 0,
  );
  const unpair = useUnpairZ21Remote(layoutId ?? 0, selectedCsId ?? 0);

  const [allowAll, setAllowAll] = useState(false);
  const [selectedVehicles, setSelectedVehicles] = useState<RosterVehicle[]>([]);
  const [handsetBrakeSecs, setHandsetBrakeSecs] = useState(
    Z21_HANDSET_BRAKE_SECS_DEFAULT,
  );
  const [actionError, setActionError] = useState<string | null>(null);
  const lastSeededKey = useRef<string | undefined>(undefined);

  const drivableVehicles = useMemo(
    () =>
      (roster.data ?? []).filter(
        (v) => v.canDrive !== false && v.dccAddress != null,
      ),
    [roster.data],
  );

  useEffect(() => {
    if (!status.data) return;
    const seedKey = status.data.paired
      ? status.data.clientKey ?? "paired"
      : status.data.pendingPairing
        ? `pending:${status.data.pendingPairing.pairingCV3}:${status.data.pendingPairing.pairingCV4}`
        : "empty";
    if (lastSeededKey.current === seedKey) return;
    lastSeededKey.current = seedKey;
    setAllowAll(status.data.allowAllVehicles);
    if (status.data.allowAllVehicles) {
      setSelectedVehicles([]);
      return;
    }
    const ids = new Set(
      (status.data.allowedVehicles ?? []).map((v) => v.vehicleId),
    );
    setSelectedVehicles(
      drivableVehicles.filter((v) => ids.has(v.id)),
    );
    if (status.data.handsetBrakeSecs != null) {
      setHandsetBrakeSecs(status.data.handsetBrakeSecs);
    }
  }, [status.data, drivableVehicles]);

  const pending = status.data?.pendingPairing;
  const countdown = usePairingCountdown(pending?.expiresAt);
  const brakeSecsHint =
    status.data?.handsetBrakeSecs ??
    pending?.handsetBrakeSecs ??
    handsetBrakeSecs;

  const translateError = (err: unknown): string => {
    if (err instanceof ApiError) {
      const localised = t(`errors:${err.code}` as const, { defaultValue: "" });
      if (localised) return localised;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  };

  const scopePayload = () => ({
    allowAllVehicles: allowAll,
    vehicleIds: allowAll ? [] : selectedVehicles.map((v) => v.id),
    handsetBrakeSecs,
  });

  const handleStartPairing = async () => {
    if (!selectedCsId) return;
    setActionError(null);
    try {
      await startPairing.mutateAsync(scopePayload());
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const handleSaveScope = async () => {
    if (!selectedCsId || !status.data?.paired) return;
    setActionError(null);
    try {
      await updateSession.mutateAsync({
        ...scopePayload(),
        clientKey: status.data.clientKey,
      });
      lastSeededKey.current = undefined;
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const handleCancelPairing = async () => {
    if (!selectedCsId) return;
    setActionError(null);
    try {
      await cancelPairing.mutateAsync();
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const handleUnpair = async (clientKey?: string) => {
    if (!selectedCsId) return;
    setActionError(null);
    try {
      await unpair.mutateAsync(clientKey ?? status.data?.clientKey);
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const submitting =
    startPairing.isPending ||
    cancelPairing.isPending ||
    updateSession.isPending ||
    unpair.isPending;
  const noZ21OnLayout = !stations.isLoading && z21Stations.length === 0;

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Stack direction="row" spacing={2} alignItems="center">
          <Z21Icon size={40} />
          <Box>
            <Typography variant="h4" component="h1" gutterBottom>
              {t("z21Remote:title")}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {t("z21Remote:subtitle")}
            </Typography>
          </Box>
        </Stack>

        {noZ21OnLayout && (
          <Alert severity="info">
            {t("z21Remote:alerts.noServer")}{" "}
            <Typography
              component={RouterLink}
              to="/admin/command-stations"
              variant="body2"
              sx={{ color: "inherit", fontWeight: 600 }}
            >
              {t("z21Remote:alerts.commandStationLink")}
            </Typography>
          </Alert>
        )}

        <Paper variant="outlined" sx={{ p: 3 }}>
          <Stack spacing={2}>
            <FormControl fullWidth disabled={z21Stations.length === 0}>
              <InputLabel>{t("z21Remote:fields.commandStation")}</InputLabel>
              <Select
                value={csId}
                label={t("z21Remote:fields.commandStation")}
                onChange={(e) => setCsId(Number(e.target.value))}
              >
                {z21Stations.map((s) => (
                  <MenuItem key={s.id} value={s.id}>
                    {s.name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            {status.isLoading && selectedCsId != null && (
              <Box sx={{ display: "flex", justifyContent: "center", py: 2 }}>
                <CircularProgress size={28} />
              </Box>
            )}

            {status.isError && (
              <Alert severity="error">{translateError(status.error)}</Alert>
            )}

            {status.data && (
              <Stack spacing={2}>
                <Box>
                  <Typography variant="subtitle2" color="text.secondary">
                    {t("z21Remote:sections.status")}
                  </Typography>
                  {status.data.paired ? (
                    <Stack spacing={0.5} sx={{ mt: 1 }}>
                      <Chip
                        size="small"
                        color="success"
                        label={t("z21Remote:status.paired")}
                      />
                      <Typography variant="body2">
                        {t("z21Remote:status.clientKey", {
                          key: status.data.clientKey,
                        })}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {t("z21Remote:status.pairedAt", {
                          time: formatTime(
                            status.data.pairedAt,
                            i18n.language,
                          ),
                        })}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {t("z21Remote:status.lastSeenAt", {
                          time: formatTime(
                            status.data.lastSeenAt,
                            i18n.language,
                          ),
                        })}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {t("z21Remote:status.keepaliveHint", {
                          seconds: brakeSecsHint,
                        })}
                      </Typography>
                    </Stack>
                  ) : (
                    <Typography variant="body2" sx={{ mt: 1 }}>
                      {t("z21Remote:status.notPaired")}
                    </Typography>
                  )}
                </Box>

                {pending && !status.data.paired && (
                  <Alert severity="info">
                    <Typography variant="subtitle1" fontWeight={600}>
                      {t("z21Remote:pending.title")}
                    </Typography>
                    <Typography variant="h4" component="p" sx={{ my: 1 }}>
                      CV3 = {pending.pairingCV3} · CV4 = {pending.pairingCV4}
                    </Typography>
                    <Typography
                      variant="body2"
                      sx={{ whiteSpace: "pre-line", mb: 1 }}
                    >
                      {t("z21Remote:pending.instructions")}
                    </Typography>
                    <Typography variant="body2">
                      {t("z21Remote:pending.expires", {
                        seconds: countdown ?? 0,
                      })}
                    </Typography>
                  </Alert>
                )}

                <Box>
                  <Typography variant="subtitle2" color="text.secondary">
                    {t("z21Remote:sections.scope")}
                  </Typography>
                  <FormControlLabel
                    sx={{ mt: 1 }}
                    control={
                      <Switch
                        checked={allowAll}
                        onChange={(e) => setAllowAll(e.target.checked)}
                        disabled={submitting}
                      />
                    }
                    label={t("z21Remote:fields.allowAllVehicles")}
                  />
                  {!allowAll && (
                    <Autocomplete
                      multiple
                      options={drivableVehicles}
                      value={selectedVehicles}
                      onChange={(_e, value) => setSelectedVehicles(value)}
                      getOptionLabel={(v) =>
                        `${v.name} (${v.dccAddress})`
                      }
                      isOptionEqualToValue={(a, b) => a.id === b.id}
                      renderInput={(params) => (
                        <TextField
                          {...params}
                          label={t("z21Remote:fields.vehicles")}
                          placeholder={t("z21Remote:fields.vehiclesPlaceholder")}
                        />
                      )}
                      disabled={submitting}
                      sx={{ mt: 1 }}
                    />
                  )}
                  {!status.data.paired && !pending && (
                    <TextField
                      type="number"
                      label={t("z21Remote:fields.handsetBrakeSecs")}
                      helperText={t("z21Remote:fields.handsetBrakeSecsHint", {
                        min: Z21_HANDSET_BRAKE_SECS_MIN,
                        max: Z21_HANDSET_BRAKE_SECS_MAX,
                      })}
                      value={handsetBrakeSecs}
                      onChange={(e) => {
                        const n = Number(e.target.value);
                        if (!Number.isFinite(n)) return;
                        setHandsetBrakeSecs(
                          Math.min(
                            Z21_HANDSET_BRAKE_SECS_MAX,
                            Math.max(Z21_HANDSET_BRAKE_SECS_MIN, n),
                          ),
                        );
                      }}
                      inputProps={{
                        min: Z21_HANDSET_BRAKE_SECS_MIN,
                        max: Z21_HANDSET_BRAKE_SECS_MAX,
                        step: 1,
                      }}
                      disabled={submitting}
                      sx={{ mt: 2 }}
                      fullWidth
                    />
                  )}
                </Box>

                <Stack direction={{ xs: "column", sm: "row" }} spacing={1}>
                  {!status.data.paired && !pending && (
                    <Button
                      variant="contained"
                      onClick={handleStartPairing}
                      disabled={
                        submitting ||
                        selectedCsId == null ||
                        (!allowAll && selectedVehicles.length === 0) ||
                        handsetBrakeSecs < Z21_HANDSET_BRAKE_SECS_MIN ||
                        handsetBrakeSecs > Z21_HANDSET_BRAKE_SECS_MAX
                      }
                    >
                      {t("z21Remote:actions.generatePairing")}
                    </Button>
                  )}
                  {pending && !status.data.paired && (
                    <Button
                      variant="outlined"
                      color="error"
                      onClick={handleCancelPairing}
                      disabled={submitting}
                    >
                      {t("z21Remote:actions.cancelPairing")}
                    </Button>
                  )}
                  {status.data.paired && (
                    <>
                      <Button
                        variant="outlined"
                        onClick={handleSaveScope}
                        disabled={
                          submitting ||
                          (!allowAll && selectedVehicles.length === 0)
                        }
                      >
                        {t("common:actions.save")}
                      </Button>
                      <Button
                        variant="outlined"
                        color="error"
                        onClick={() => handleUnpair()}
                        disabled={submitting}
                      >
                        {t("z21Remote:actions.removePairedHandset")}
                      </Button>
                    </>
                  )}
                </Stack>
              </Stack>
            )}

            {actionError && <Alert severity="error">{actionError}</Alert>}
          </Stack>
        </Paper>

        {selectedCsId != null && (
          <Paper variant="outlined" sx={{ p: 3 }}>
            <Stack spacing={2}>
              <Typography variant="h6">{t("z21Remote:sections.clients")}</Typography>
              {clients.isLoading && (
                <Box sx={{ display: "flex", justifyContent: "center", py: 2 }}>
                  <CircularProgress size={28} />
                </Box>
              )}
              {clients.isError && (
                <Alert severity="error">{translateError(clients.error)}</Alert>
              )}
              {clients.data && clients.data.clients.length === 0 && (
                <Typography variant="body2" color="text.secondary">
                  {t("z21Remote:clients.empty")}
                </Typography>
              )}
              {clients.data && clients.data.clients.length > 0 && (
                <TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>{t("z21Remote:clients.columns.endpoint")}</TableCell>
                        <TableCell>{t("z21Remote:clients.columns.paired")}</TableCell>
                        <TableCell>{t("z21Remote:clients.columns.user")}</TableCell>
                        <TableCell>{t("z21Remote:clients.columns.lastSeen")}</TableCell>
                        <TableCell>{t("z21Remote:clients.columns.connected")}</TableCell>
                        <TableCell>{t("z21Remote:clients.columns.status")}</TableCell>
                        <TableCell align="right">
                          {t("z21Remote:clients.columns.actions")}
                        </TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {clients.data.clients.map((row: Z21RemoteClient) => (
                        <TableRow key={row.clientKey}>
                          <TableCell>
                            {row.ip}:{row.port}
                          </TableCell>
                          <TableCell>
                            {row.paired
                              ? t("z21Remote:clients.pairedYes")
                              : t("z21Remote:clients.pairedNo")}
                          </TableCell>
                          <TableCell>
                            {row.userLogin ?? (row.userId ? `#${row.userId}` : "—")}
                          </TableCell>
                          <TableCell>
                            {formatTime(row.lastSeenAt, i18n.language)}
                          </TableCell>
                          <TableCell>
                            {formatDuration(now - row.connectedAt)}
                          </TableCell>
                          <TableCell>
                            {row.idleBraked && (
                              <Chip
                                size="small"
                                color="warning"
                                label={t("z21Remote:clients.idleBraked")}
                              />
                            )}
                            {row.paired && clients.data.ipStickiness && row.sessionExpiresAt != null && (
                              <Typography variant="caption" display="block" color="text.secondary">
                                {t("z21Remote:clients.sessionExpires", {
                                  time: formatTime(row.sessionExpiresAt, i18n.language),
                                })}
                              </Typography>
                            )}
                          </TableCell>
                          <TableCell align="right">
                            {row.paired && row.userId === me?.id && (
                              <Button
                                size="small"
                                color="error"
                                onClick={() => handleUnpair(row.clientKey)}
                                disabled={submitting}
                              >
                                {t("z21Remote:actions.removePairedHandset")}
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
            </Stack>
          </Paper>
        )}
      </Stack>
    </Container>
  );
}
