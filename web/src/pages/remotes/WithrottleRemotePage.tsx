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
  REMOTE_HANDSET_BRAKE_SECS_DEFAULT,
  REMOTE_HANDSET_BRAKE_SECS_MAX,
  REMOTE_HANDSET_BRAKE_SECS_MIN,
  useCancelWithrottlePairing,
  useStartWithrottlePairing,
  useUnpairWithrottleRemote,
  useUpdateWithrottleRemoteSession,
  useWithrottleRemoteStatus,
} from "../../api/remotes";

function formatTime(ms: number | undefined, locale: string): string {
  if (ms == null || ms <= 0) return "—";
  return new Date(ms).toLocaleString(locale);
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

export default function WithrottleRemotePage() {
  const { t, i18n } = useTranslation(["withrottleRemote", "common", "errors"]);
  const me = useMe().data;
  const layoutId = me?.layoutId ?? null;

  const stations = useLayoutCommandStations(layoutId);
  const wtStations = useMemo(
    () => (stations.data ?? []).filter((s) => s.withrottleServerEnabled),
    [stations.data],
  );

  const [csId, setCsId] = useState<number | "">("");
  useEffect(() => {
    if (wtStations.length === 0) {
      setCsId("");
      return;
    }
    if (csId === "" || !wtStations.some((s) => s.id === csId)) {
      setCsId(wtStations[0].id);
    }
  }, [wtStations, csId]);

  const selectedCsId = typeof csId === "number" ? csId : null;
  const status = useWithrottleRemoteStatus(layoutId, selectedCsId);
  const roster = useLayoutVehicles(layoutId);

  const startPairing = useStartWithrottlePairing(layoutId ?? 0, selectedCsId ?? 0);
  const cancelPairing = useCancelWithrottlePairing(layoutId ?? 0, selectedCsId ?? 0);
  const updateSession = useUpdateWithrottleRemoteSession(
    layoutId ?? 0,
    selectedCsId ?? 0,
  );
  const unpair = useUnpairWithrottleRemote(layoutId ?? 0, selectedCsId ?? 0);

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
      ? status.data.clientKey ?? "paired"
      : pending?.protocol === REMOTE_PROTOCOL_WITHROTTLE
        ? `pending:${pending.pairingCode ?? pending.displayLabel}`
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
    setSelectedVehicles(drivableVehicles.filter((v) => ids.has(v.id)));
    if (status.data.handsetBrakeSecs != null) {
      setHandsetBrakeSecs(status.data.handsetBrakeSecs);
    }
  }, [status.data, drivableVehicles]);

  const pending =
    status.data?.pendingPairing?.protocol === REMOTE_PROTOCOL_WITHROTTLE
      ? status.data.pendingPairing
      : undefined;
  const countdown = usePairingCountdown(pending?.expiresAt);
  const brakeSecsHint =
    status.data?.handsetBrakeSecs ??
    pending?.handsetBrakeSecs ??
    handsetBrakeSecs;
  const pairedWithrottle =
    status.data?.paired &&
    status.data.protocol === REMOTE_PROTOCOL_WITHROTTLE;

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

  const submitting =
    startPairing.isPending ||
    cancelPairing.isPending ||
    updateSession.isPending ||
    unpair.isPending;

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Box>
          <Typography variant="h4" component="h1" gutterBottom>
            {t("withrottleRemote:title")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("withrottleRemote:subtitle")}
          </Typography>
        </Box>

        {!stations.isLoading && wtStations.length === 0 && (
          <Alert severity="info">
            {t("withrottleRemote:alerts.noServer")}{" "}
            <Typography
              component={RouterLink}
              to="/admin/command-stations"
              variant="body2"
              sx={{ color: "inherit", fontWeight: 600 }}
            >
              {t("withrottleRemote:alerts.commandStationLink")}
            </Typography>
          </Alert>
        )}

        <Paper variant="outlined" sx={{ p: 3 }}>
          <Stack spacing={2}>
            <FormControl fullWidth disabled={wtStations.length === 0}>
              <InputLabel>{t("withrottleRemote:fields.commandStation")}</InputLabel>
              <Select
                value={csId}
                label={t("withrottleRemote:fields.commandStation")}
                onChange={(e) => setCsId(Number(e.target.value))}
              >
                {wtStations.map((s) => (
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

            {actionError && <Alert severity="error">{actionError}</Alert>}

            {status.data && (
              <Stack spacing={2}>
                <Box>
                  <Typography variant="subtitle2" color="text.secondary">
                    {t("withrottleRemote:sections.status")}
                  </Typography>
                  {pairedWithrottle ? (
                    <Stack spacing={0.5} sx={{ mt: 1 }}>
                      <Chip
                        size="small"
                        color="success"
                        label={t("withrottleRemote:status.paired")}
                      />
                      <Typography variant="body2">
                        {t("withrottleRemote:status.clientKey", {
                          key: status.data.clientKey,
                        })}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {t("withrottleRemote:status.pairedAt", {
                          time: formatTime(status.data.pairedAt, i18n.language),
                        })}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {t("withrottleRemote:status.lastSeenAt", {
                          time: formatTime(
                            status.data.lastSeenAt,
                            i18n.language,
                          ),
                        })}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {t("withrottleRemote:status.keepaliveHint", {
                          seconds: brakeSecsHint,
                        })}
                      </Typography>
                    </Stack>
                  ) : (
                    <Typography variant="body2" sx={{ mt: 1 }}>
                      {t("withrottleRemote:status.notPaired")}
                    </Typography>
                  )}
                </Box>

                {pending && !pairedWithrottle && (
                  <Alert severity="info">
                    <Typography variant="subtitle1" fontWeight={600}>
                      {t("withrottleRemote:pending.title")}
                    </Typography>
                    <Typography variant="h4" component="p" sx={{ my: 1 }}>
                      {pending.displayLabel}
                    </Typography>
                    <Typography
                      variant="body2"
                      sx={{ whiteSpace: "pre-line", mb: 1 }}
                    >
                      {t("withrottleRemote:pending.instructions")}
                    </Typography>
                    <Typography variant="body2">
                      {t("withrottleRemote:pending.expires", {
                        seconds: countdown ?? 0,
                      })}
                    </Typography>
                  </Alert>
                )}

                <Box>
                  <Typography variant="subtitle2" color="text.secondary">
                    {t("withrottleRemote:sections.scope")}
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
                    label={t("withrottleRemote:fields.allowAllVehicles")}
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
                          label={t("withrottleRemote:fields.vehicles")}
                          placeholder={t(
                            "withrottleRemote:fields.vehiclesPlaceholder",
                          )}
                        />
                      )}
                      disabled={submitting}
                      sx={{ mt: 1 }}
                    />
                  )}
                  {!pairedWithrottle && !pending && (
                    <TextField
                      type="number"
                      label={t("withrottleRemote:fields.handsetBrakeSecs")}
                      helperText={t(
                        "withrottleRemote:fields.handsetBrakeSecsHint",
                        {
                          min: REMOTE_HANDSET_BRAKE_SECS_MIN,
                          max: REMOTE_HANDSET_BRAKE_SECS_MAX,
                        },
                      )}
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
                  {!pairedWithrottle && !pending && (
                    <Button
                      variant="contained"
                      onClick={async () => {
                        if (!selectedCsId) return;
                        setActionError(null);
                        try {
                          await startPairing.mutateAsync(scopePayload());
                        } catch (err) {
                          setActionError(translateError(err));
                        }
                      }}
                      disabled={
                        submitting ||
                        selectedCsId == null ||
                        (!allowAll && selectedVehicles.length === 0)
                      }
                    >
                      {t("withrottleRemote:actions.generatePairing")}
                    </Button>
                  )}
                  {pending && !pairedWithrottle && (
                    <Button
                      variant="outlined"
                      color="warning"
                      onClick={async () => {
                        setActionError(null);
                        try {
                          await cancelPairing.mutateAsync();
                        } catch (err) {
                          setActionError(translateError(err));
                        }
                      }}
                      disabled={submitting}
                    >
                      {t("withrottleRemote:actions.cancelPairing")}
                    </Button>
                  )}
                  {pairedWithrottle && (
                    <>
                      <Button
                        variant="contained"
                        onClick={async () => {
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
                        }}
                        disabled={submitting}
                      >
                        {t("withrottleRemote:actions.saveScope")}
                      </Button>
                      <Button
                        variant="outlined"
                        color="error"
                        onClick={async () => {
                          setActionError(null);
                          try {
                            await unpair.mutateAsync(status.data?.clientKey);
                          } catch (err) {
                            setActionError(translateError(err));
                          }
                        }}
                        disabled={submitting}
                      >
                        {t("withrottleRemote:actions.unpair")}
                      </Button>
                    </>
                  )}
                </Stack>
              </Stack>
            )}
          </Stack>
        </Paper>
      </Stack>
    </Container>
  );
}
