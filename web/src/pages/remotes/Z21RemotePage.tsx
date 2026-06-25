import { useEffect, useMemo, useState } from "react";
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
import PhoneAndroidIcon from "@mui/icons-material/PhoneAndroid";
import { Link as RouterLink } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { ApiError } from "../../api/client";
import { useMe } from "../../api/auth";
import { useLayoutCommandStations } from "../../api/command_stations";
import { useLayoutVehicles, type RosterVehicle } from "../../api/vehicles";
import {
  useStartZ21Pairing,
  useUnpairZ21Remote,
  useUpdateZ21RemoteSession,
  useZ21RemoteStatus,
} from "../../api/z21_remote";

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
  const roster = useLayoutVehicles(layoutId);

  const startPairing = useStartZ21Pairing(layoutId ?? 0, selectedCsId ?? 0);
  const updateSession = useUpdateZ21RemoteSession(
    layoutId ?? 0,
    selectedCsId ?? 0,
  );
  const unpair = useUnpairZ21Remote(layoutId ?? 0, selectedCsId ?? 0);

  const [allowAll, setAllowAll] = useState(false);
  const [selectedVehicles, setSelectedVehicles] = useState<RosterVehicle[]>([]);
  const [actionError, setActionError] = useState<string | null>(null);

  const drivableVehicles = useMemo(
    () =>
      (roster.data ?? []).filter(
        (v) => v.canDrive !== false && v.dccAddress != null,
      ),
    [roster.data],
  );

  useEffect(() => {
    if (!status.data) return;
    setAllowAll(status.data.allowAllVehicles);
    if (status.data.allowAllVehicles) {
      setSelectedVehicles([]);
      return;
    }
    const ids = new Set(
      status.data.allowedVehicles.map((v) => v.vehicleId),
    );
    setSelectedVehicles(
      drivableVehicles.filter((v) => ids.has(v.id)),
    );
  }, [status.data, drivableVehicles]);

  const pending = status.data?.pendingPairing;
  const countdown = usePairingCountdown(pending?.expiresAt);

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
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const handleUnpair = async () => {
    if (!selectedCsId) return;
    setActionError(null);
    try {
      await unpair.mutateAsync(status.data?.clientKey);
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const submitting =
    startPairing.isPending || updateSession.isPending || unpair.isPending;
  const noZ21OnLayout = !stations.isLoading && z21Stations.length === 0;

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Stack direction="row" spacing={2} alignItems="center">
          <PhoneAndroidIcon color="primary" fontSize="large" />
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
                        {t("z21Remote:status.keepaliveHint")}
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
                </Box>

                <Stack direction={{ xs: "column", sm: "row" }} spacing={1}>
                  {!status.data.paired && !pending && (
                    <Button
                      variant="contained"
                      onClick={handleStartPairing}
                      disabled={
                        submitting ||
                        selectedCsId == null ||
                        (!allowAll && selectedVehicles.length === 0)
                      }
                    >
                      {t("z21Remote:actions.generatePairing")}
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
                        onClick={handleUnpair}
                        disabled={submitting}
                      >
                        {t("z21Remote:actions.unpair")}
                      </Button>
                    </>
                  )}
                </Stack>
              </Stack>
            )}

            {actionError && <Alert severity="error">{actionError}</Alert>}
          </Stack>
        </Paper>
      </Stack>
    </Container>
  );
}
