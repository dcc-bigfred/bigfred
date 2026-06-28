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
  Tab,
  Tabs,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from "@mui/material";
import { Link as RouterLink } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ApiError } from "../../api/client";
import { useMe } from "../../api/auth";
import { useLayoutCommandStations } from "../../api/command_stations";
import { useLayoutVehicles, type RosterVehicle } from "../../api/vehicles";
import {
  REMOTE_PROTOCOL_WITHROTTLE,
  REMOTE_PROTOCOL_Z21,
  REMOTE_HANDSET_BRAKE_SECS_DEFAULT,
  REMOTE_HANDSET_BRAKE_SECS_MAX,
  REMOTE_HANDSET_BRAKE_SECS_MIN,
  useCancelRemotePairing,
  useRemoteClients,
  useRemoteStatus,
  useStartRemotePairing,
  useUnpairRemote,
  useUpdateRemoteSession,
  type RemoteClient,
} from "../../api/remotes";

type RemoteProtocol = typeof REMOTE_PROTOCOL_Z21 | typeof REMOTE_PROTOCOL_WITHROTTLE;

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

function protocolLabelKey(protocol: string): "remotes:protocol.z21" | "remotes:protocol.withrottle" {
  return protocol === REMOTE_PROTOCOL_WITHROTTLE
    ? "remotes:protocol.withrottle"
    : "remotes:protocol.z21";
}

export default function RemotesPage() {
  const { t, i18n } = useTranslation(["remotes", "common", "errors"]);
  const me = useMe().data;
  const layoutId = me?.layoutId ?? null;

  const stations = useLayoutCommandStations(layoutId);
  const remoteStations = useMemo(
    () =>
      (stations.data ?? []).filter(
        (s) => s.z21ServerEnabled || s.withrottleServerEnabled,
      ),
    [stations.data],
  );

  const [csId, setCsId] = useState<number | "">("");
  const [protocol, setProtocol] = useState<RemoteProtocol>(REMOTE_PROTOCOL_Z21);
  const sessionProtocolRef = useRef<string | undefined>(undefined);

  useEffect(() => {
    if (remoteStations.length === 0) {
      setCsId("");
      return;
    }
    if (csId === "" || !remoteStations.some((s) => s.id === csId)) {
      setCsId(remoteStations[0].id);
    }
  }, [remoteStations, csId]);

  const selectedCsId = typeof csId === "number" ? csId : null;

  useEffect(() => {
    sessionProtocolRef.current = undefined;
  }, [selectedCsId]);
  const status = useRemoteStatus(layoutId, selectedCsId);
  const clients = useRemoteClients(layoutId, selectedCsId);
  const now = useNowTick((clients.data?.clients.length ?? 0) > 0);
  const roster = useLayoutVehicles(layoutId);

  const enabledProtocols = useMemo(() => {
    const fromStatus = (status.data?.availableProtocols ?? [])
      .filter((p) => p.enabled)
      .map((p) => p.protocol);
    if (fromStatus.length > 0) return fromStatus;
    const cs = remoteStations.find((s) => s.id === selectedCsId);
    if (!cs) return [];
    const out: string[] = [];
    if (cs.z21ServerEnabled) out.push(REMOTE_PROTOCOL_Z21);
    if (cs.withrottleServerEnabled) out.push(REMOTE_PROTOCOL_WITHROTTLE);
    return out;
  }, [status.data?.availableProtocols, remoteStations, selectedCsId]);

  useEffect(() => {
    const hasSession = status.data?.paired || status.data?.pendingPairing;
    const sessionProtocol = hasSession ? status.data?.protocol : undefined;

    if (sessionProtocol && enabledProtocols.includes(sessionProtocol)) {
      if (sessionProtocolRef.current !== sessionProtocol) {
        sessionProtocolRef.current = sessionProtocol;
        setProtocol(sessionProtocol as RemoteProtocol);
        return;
      }
    } else {
      sessionProtocolRef.current = undefined;
    }

    if (enabledProtocols.length > 0 && !enabledProtocols.includes(protocol)) {
      setProtocol(enabledProtocols[0] as RemoteProtocol);
    }
  }, [
    status.data?.protocol,
    status.data?.paired,
    status.data?.pendingPairing,
    enabledProtocols,
    protocol,
  ]);

  const startPairing = useStartRemotePairing(
    layoutId ?? 0,
    selectedCsId ?? 0,
    protocol,
  );
  const cancelPairing = useCancelRemotePairing(layoutId ?? 0, selectedCsId ?? 0);
  const updateSession = useUpdateRemoteSession(layoutId ?? 0, selectedCsId ?? 0);
  const unpair = useUnpairRemote(layoutId ?? 0, selectedCsId ?? 0);

  const [allowAll, setAllowAll] = useState(false);
  const [selectedVehicles, setSelectedVehicles] = useState<RosterVehicle[]>([]);
  const [handsetBrakeSecs, setHandsetBrakeSecs] = useState(
    REMOTE_HANDSET_BRAKE_SECS_DEFAULT,
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
    const pending = status.data.pendingPairing;
    const seedKey = status.data.paired
      ? `${status.data.protocol}:${status.data.clientKey ?? "paired"}`
      : pending
        ? `pending:${pending.protocol}:${pending.pairingCV3 ?? ""}:${pending.pairingCV4 ?? ""}:${pending.pairingCode ?? pending.displayLabel}`
        : "empty";
    if (lastSeededKey.current === seedKey) return;
    lastSeededKey.current = seedKey;
    setAllowAll(status.data.allowAllVehicles);
    if (status.data.allowAllVehicles) {
      setSelectedVehicles([]);
    } else {
      const ids = new Set(
        (status.data.allowedVehicles ?? []).map((v) => v.vehicleId),
      );
      setSelectedVehicles(drivableVehicles.filter((v) => ids.has(v.id)));
    }
    if (status.data.handsetBrakeSecs != null) {
      setHandsetBrakeSecs(status.data.handsetBrakeSecs);
    }
  }, [status.data, drivableVehicles]);

  const pairedForProtocol =
    status.data?.paired === true && status.data.protocol === protocol;
  const pending =
    status.data?.pendingPairing?.protocol === protocol
      ? status.data.pendingPairing
      : undefined;
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
    if (!selectedCsId || !pairedForProtocol) return;
    setActionError(null);
    try {
      await updateSession.mutateAsync({
        ...scopePayload(),
        clientKey: status.data?.clientKey,
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
  const noRemoteOnLayout = !stations.isLoading && remoteStations.length === 0;
  const showProtocolPicker = enabledProtocols.length > 1;

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Box>
          <Typography variant="h4" component="h1" gutterBottom>
            {t("remotes:title")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("remotes:subtitle")}
          </Typography>
        </Box>

        {noRemoteOnLayout && (
          <Alert severity="info">
            {t("remotes:alerts.noServer")}{" "}
            <Typography
              component={RouterLink}
              to="/admin/command-stations"
              variant="body2"
              sx={{ color: "inherit", fontWeight: 600 }}
            >
              {t("remotes:alerts.commandStationLink")}
            </Typography>
          </Alert>
        )}

        <Paper variant="outlined" sx={{ p: 3 }}>
          <Stack spacing={2}>
            <FormControl fullWidth disabled={remoteStations.length === 0}>
              <InputLabel>{t("remotes:fields.commandStation")}</InputLabel>
              <Select
                value={csId}
                label={t("remotes:fields.commandStation")}
                onChange={(e) => setCsId(Number(e.target.value))}
              >
                {remoteStations.map((s) => (
                  <MenuItem key={s.id} value={s.id}>
                    {s.name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            {showProtocolPicker && (
              <Box>
                <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
                  {t("remotes:sections.protocol")}
                </Typography>
                <Tabs
                  value={protocol}
                  onChange={(_e, value: RemoteProtocol) => setProtocol(value)}
                  variant="fullWidth"
                >
                  {enabledProtocols.map((p) => {
                    const hasPending =
                      status.data?.pendingPairing?.protocol === p && !status.data?.paired;
                    return (
                      <Tab
                        key={p}
                        value={p}
                        label={
                          <Stack direction="row" alignItems="center" spacing={1}>
                            <span>{t(protocolLabelKey(p))}</span>
                            {hasPending && p !== protocol && (
                              <Chip
                                size="small"
                                color="warning"
                                label={t("remotes:protocol.pendingBadge")}
                              />
                            )}
                          </Stack>
                        }
                        disabled={submitting}
                      />
                    );
                  })}
                </Tabs>
              </Box>
            )}

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
                    {t("remotes:sections.status")}
                  </Typography>
                  {pairedForProtocol ? (
                    <Stack spacing={0.5} sx={{ mt: 1 }}>
                      <Chip
                        size="small"
                        color="success"
                        label={t("remotes:status.paired")}
                      />
                      <Typography variant="body2">
                        {t("remotes:status.clientKey", {
                          key: status.data.clientKey,
                        })}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {t("remotes:status.pairedAt", {
                          time: formatTime(status.data.pairedAt, i18n.language),
                        })}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {t("remotes:status.lastSeenAt", {
                          time: formatTime(
                            status.data.lastSeenAt,
                            i18n.language,
                          ),
                        })}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {t("remotes:status.keepaliveHint", {
                          seconds: brakeSecsHint,
                        })}
                      </Typography>
                    </Stack>
                  ) : (
                    <Typography variant="body2" sx={{ mt: 1 }}>
                      {t("remotes:status.notPaired")}
                    </Typography>
                  )}
                </Box>

                {pending && !pairedForProtocol && (
                  <Alert severity="info">
                    <Typography variant="subtitle1" fontWeight={600}>
                      {t("remotes:pending.title")}
                    </Typography>
                    <Typography variant="h4" component="p" sx={{ my: 1 }}>
                      {protocol === REMOTE_PROTOCOL_Z21 &&
                      pending.pairingCV3 != null &&
                      pending.pairingCV4 != null
                        ? `CV3 = ${pending.pairingCV3} · CV4 = ${pending.pairingCV4}`
                        : pending.displayLabel}
                    </Typography>
                    <Typography
                      variant="body2"
                      sx={{ whiteSpace: "pre-line", mb: 1 }}
                    >
                      {t(
                        protocol === REMOTE_PROTOCOL_Z21
                          ? "remotes:protocolPending.z21.instructions"
                          : "remotes:protocolPending.withrottle.instructions",
                      )}
                    </Typography>
                    <Typography variant="body2">
                      {t("remotes:pending.expires", {
                        seconds: countdown ?? 0,
                      })}
                    </Typography>
                  </Alert>
                )}

                <Box>
                  <Typography variant="subtitle2" color="text.secondary">
                    {t("remotes:sections.scope")}
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
                    label={t("remotes:fields.allowAllVehicles")}
                  />
                  {!allowAll && (
                    <Autocomplete
                      multiple
                      options={drivableVehicles}
                      value={selectedVehicles}
                      onChange={(_e, value) => setSelectedVehicles(value)}
                      getOptionLabel={(v) => `${v.name} (${v.dccAddress})`}
                      isOptionEqualToValue={(a, b) => a.id === b.id}
                      renderInput={(params) => (
                        <TextField
                          {...params}
                          label={t("remotes:fields.vehicles")}
                          placeholder={t("remotes:fields.vehiclesPlaceholder")}
                        />
                      )}
                      disabled={submitting}
                      sx={{ mt: 1 }}
                    />
                  )}
                  {!pairedForProtocol && !pending && (
                    <TextField
                      type="number"
                      label={t("remotes:fields.handsetBrakeSecs")}
                      helperText={t("remotes:fields.handsetBrakeSecsHint", {
                        min: REMOTE_HANDSET_BRAKE_SECS_MIN,
                        max: REMOTE_HANDSET_BRAKE_SECS_MAX,
                      })}
                      value={handsetBrakeSecs}
                      onChange={(e) => {
                        const n = Number(e.target.value);
                        if (!Number.isFinite(n)) return;
                        setHandsetBrakeSecs(
                          Math.min(
                            REMOTE_HANDSET_BRAKE_SECS_MAX,
                            Math.max(REMOTE_HANDSET_BRAKE_SECS_MIN, n),
                          ),
                        );
                      }}
                      inputProps={{
                        min: REMOTE_HANDSET_BRAKE_SECS_MIN,
                        max: REMOTE_HANDSET_BRAKE_SECS_MAX,
                        step: 1,
                      }}
                      disabled={submitting}
                      sx={{ mt: 2 }}
                      fullWidth
                    />
                  )}
                </Box>

                <Stack direction={{ xs: "column", sm: "row" }} spacing={1}>
                  {!pairedForProtocol && !pending && (
                    <Button
                      variant="contained"
                      onClick={handleStartPairing}
                      disabled={
                        submitting ||
                        selectedCsId == null ||
                        !enabledProtocols.includes(protocol) ||
                        (!allowAll && selectedVehicles.length === 0) ||
                        handsetBrakeSecs < REMOTE_HANDSET_BRAKE_SECS_MIN ||
                        handsetBrakeSecs > REMOTE_HANDSET_BRAKE_SECS_MAX
                      }
                    >
                      {t("remotes:actions.generatePairing")}
                    </Button>
                  )}
                  {pending && !pairedForProtocol && (
                    <Button
                      variant="outlined"
                      color="error"
                      onClick={handleCancelPairing}
                      disabled={submitting}
                    >
                      {t("remotes:actions.cancelPairing")}
                    </Button>
                  )}
                  {pairedForProtocol && (
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
                        {t("remotes:actions.removePairedHandset")}
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
              <Typography variant="h6">{t("remotes:sections.clients")}</Typography>
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
                  {t("remotes:clients.empty")}
                </Typography>
              )}
              {clients.data && clients.data.clients.length > 0 && (
                <TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>
                          {t("remotes:clients.columns.protocol")}
                        </TableCell>
                        <TableCell>
                          {t("remotes:clients.columns.endpoint")}
                        </TableCell>
                        <TableCell>
                          {t("remotes:clients.columns.paired")}
                        </TableCell>
                        <TableCell>
                          {t("remotes:clients.columns.user")}
                        </TableCell>
                        <TableCell>
                          {t("remotes:clients.columns.lastSeen")}
                        </TableCell>
                        <TableCell>
                          {t("remotes:clients.columns.connected")}
                        </TableCell>
                        <TableCell>
                          {t("remotes:clients.columns.status")}
                        </TableCell>
                        <TableCell align="right">
                          {t("remotes:clients.columns.actions")}
                        </TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {clients.data.clients.map((row: RemoteClient) => (
                        <TableRow key={row.clientKey}>
                          <TableCell>
                            {t(protocolLabelKey(row.protocol ?? REMOTE_PROTOCOL_Z21))}
                          </TableCell>
                          <TableCell>
                            {row.ip}:{row.port}
                          </TableCell>
                          <TableCell>
                            {row.paired
                              ? t("remotes:clients.pairedYes")
                              : t("remotes:clients.pairedNo")}
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
                                label={t("remotes:clients.idleBraked")}
                              />
                            )}
                            {row.paired &&
                              clients.data.ipStickiness &&
                              row.sessionExpiresAt != null && (
                                <Typography
                                  variant="caption"
                                  display="block"
                                  color="text.secondary"
                                >
                                  {t("remotes:clients.sessionExpires", {
                                    time: formatTime(
                                      row.sessionExpiresAt,
                                      i18n.language,
                                    ),
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
                                {t("remotes:actions.removePairedHandset")}
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
