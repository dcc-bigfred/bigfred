import { useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  FormControlLabel,
  FormHelperText,
  Link,
  Paper,
  Stack,
  Step,
  StepLabel,
  Stepper,
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
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { Link as RouterLink, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";

import { ApiError, apiFetch } from "../../api/client";
import { useMe } from "../../api/auth";
import {
  DEFAULT_COMMAND_STATION_DEADMAN_SECS,
  DEFAULT_COMMAND_STATION_HEARTBEAT_SECS,
  DEFAULT_COMMAND_STATION_IDLE_TIMEOUT_SECS,
  DEFAULT_COMMAND_STATION_MAX_LOCONET_SLOTS,
  DEFAULT_COMMAND_STATION_POLL_INTERVAL_MS,
  DEFAULT_COMMAND_STATION_SPEED_STEPS,
  commandStationScanWsUrl,
  isSerialAutodetectUri,
  kindFromConnectionUri,
  layoutCommandStationsQueryKey,
  useCreateCommandStation,
  useSetLayoutCommandStations,
  type CommandStation,
  type DetectedConnection,
  type CommandStationKind,
  type ScanWsFrame,
} from "../../api/command_stations";

function isLoconetKind(kind: CommandStationKind): boolean {
  return kind === "loconet_serial" || kind === "loconet_tcp";
}

type WizardStepId = "select" | "remotes" | "bootstop" | "slots" | "layout";

export default function ConnectionWizardPage() {
  const { t } = useTranslation(["commandStation", "common", "errors"]);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const me = useMe();
  const create = useCreateCommandStation();
  const setLayoutStations = useSetLayoutCommandStations();

  const [scanGeneration, setScanGeneration] = useState(0);
  const [scanning, setScanning] = useState(false);
  const [rows, setRows] = useState<DetectedConnection[]>([]);
  const [scanError, setScanError] = useState<string | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const scanCancelledRef = useRef(false);

  const [stepIndex, setStepIndex] = useState(0);
  const [selected, setSelected] = useState<DetectedConnection | null>(null);
  const [nameInput, setNameInput] = useState("");
  const [z21ServerEnabled, setZ21ServerEnabled] = useState(true);
  const [withrottleServerEnabled, setWithrottleServerEnabled] = useState(true);
  const [bootStopEnabled, setBootStopEnabled] = useState(false);
  const [allocatePhysicalSlots, setAllocatePhysicalSlots] = useState(true);
  const [attachToLayout, setAttachToLayout] = useState(true);
  const [actionError, setActionError] = useState<string | null>(null);

  const layoutId = me.data?.layoutId ?? null;
  const layoutName = me.data?.layoutName ?? "";

  const selectedKind = selected
    ? kindFromConnectionUri(selected.uri)
    : null;
  const showSlotsStep = selectedKind != null && isLoconetKind(selectedKind);

  const steps = useMemo((): WizardStepId[] => {
    const base: WizardStepId[] = ["select", "remotes", "bootstop"];
    if (showSlotsStep) base.push("slots");
    base.push("layout");
    return base;
  }, [showSlotsStep]);

  const activeStep = steps[Math.min(stepIndex, steps.length - 1)] ?? "select";

  useEffect(() => {
    if (stepIndex >= steps.length) {
      setStepIndex(steps.length - 1);
    }
  }, [stepIndex, steps.length]);

  // One-shot WebSocket scan — do not use useWsConnection (auto-reconnect).
  useEffect(() => {
    let disposed = false;
    scanCancelledRef.current = false;
    setScanning(true);
    setScanError(null);
    setRows([]);

    const socket = new WebSocket(commandStationScanWsUrl());
    socketRef.current = socket;

    socket.onmessage = (ev) => {
      if (disposed) return;
      let msg: ScanWsFrame;
      try {
        msg = JSON.parse(String(ev.data)) as ScanWsFrame;
      } catch {
        return;
      }
      if (msg.type === "connection" && msg.uri) {
        setRows((prev) => {
          if (prev.some((r) => r.uri === msg.uri)) return prev;
          return [...prev, { name: msg.name, uri: msg.uri }];
        });
        return;
      }
      if (msg.type === "error") {
        const detail = (msg.detail ?? "").trim();
        if (detail === "scan already running") {
          setScanError(t("commandStation:admin.wizard.scanAlreadyRunning"));
        } else if (detail) {
          setScanError(
            `${t("errors:scan_failed")}\n${detail}`,
          );
        } else {
          setScanError(t("errors:scan_failed"));
        }
        return;
      }
      if (msg.type === "done") {
        setScanning(false);
      }
    };

    socket.onerror = () => {
      if (disposed || scanCancelledRef.current) return;
      setScanError((prev) => prev ?? t("errors:scan_failed"));
    };

    socket.onclose = () => {
      if (disposed) return;
      setScanning(false);
      if (socketRef.current === socket) socketRef.current = null;
    };

    return () => {
      disposed = true;
      socket.onopen = null;
      socket.onmessage = null;
      socket.onerror = null;
      socket.onclose = null;
      if (
        socket.readyState === WebSocket.OPEN ||
        socket.readyState === WebSocket.CONNECTING
      ) {
        socket.close();
      }
      if (socketRef.current === socket) socketRef.current = null;
    };
  }, [scanGeneration, t]);

  const translateError = (err: unknown): string => {
    if (err instanceof ApiError) {
      const localised = t(`errors:${err.code}` as const, { defaultValue: "" });
      if (localised) return localised;
      return t("errors:unknown", { code: err.code });
    }
    if (err instanceof Error) return err.message;
    return t("errors:network");
  };

  const cancelScan = () => {
    if (!scanning) return;
    scanCancelledRef.current = true;
    const socket = socketRef.current;
    if (
      socket &&
      (socket.readyState === WebSocket.OPEN ||
        socket.readyState === WebSocket.CONNECTING)
    ) {
      socket.close();
    }
    setScanning(false);
  };

  const rescan = () => {
    if (scanning) return;
    setSelected(null);
    setNameInput("");
    setScanGeneration((g) => g + 1);
  };

  const selectRow = (row: DetectedConnection) => {
    setSelected(row);
    setNameInput(row.name);
  };

  const canNext = (): boolean => {
    if (activeStep === "select") {
      return selected != null && nameInput.trim().length > 0;
    }
    return true;
  };

  const goNext = () => {
    if (!canNext()) return;
    if (stepIndex < steps.length - 1) {
      setStepIndex((i) => i + 1);
      return;
    }
    void finish();
  };

  const goBack = () => {
    if (stepIndex > 0) setStepIndex((i) => i - 1);
  };

  const finish = async () => {
    if (!selected) return;
    setActionError(null);
    const kind = kindFromConnectionUri(selected.uri);
    try {
      const created = await create.mutateAsync({
        name: nameInput.trim(),
        kind,
        connectionUri: selected.uri,
        speedSteps: DEFAULT_COMMAND_STATION_SPEED_STEPS,
        heartbeatSecs: DEFAULT_COMMAND_STATION_HEARTBEAT_SECS,
        deadmanSecs: DEFAULT_COMMAND_STATION_DEADMAN_SECS,
        pollIntervalMs: DEFAULT_COMMAND_STATION_POLL_INTERVAL_MS,
        z21ServerEnabled,
        z21IpStickiness: false,
        withrottleServerEnabled,
        idleTimeoutSecs: DEFAULT_COMMAND_STATION_IDLE_TIMEOUT_SECS,
        bootStopEnabled,
        singleVehicleControl: false,
        ...(isLoconetKind(kind)
          ? {
              maxLoconetSlots: DEFAULT_COMMAND_STATION_MAX_LOCONET_SLOTS,
              allocatePhysicalSlots,
            }
          : {}),
      });

      if (attachToLayout && layoutId != null && layoutId > 0) {
        const existing = await qc.fetchQuery({
          queryKey: layoutCommandStationsQueryKey(layoutId),
          queryFn: () =>
            apiFetch<CommandStation[]>(
              `/api/v1/layouts/${layoutId}/command-stations`,
            ),
        });
        const ids = [
          ...existing.map((cs) => cs.id).filter((id) => id !== created.id),
          created.id,
        ];
        await setLayoutStations.mutateAsync({
          layoutId,
          commandStationIds: ids,
        });
      }
      navigate("/admin/command-stations");
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const submitting = create.isPending || setLayoutStations.isPending;

  const stepLabel = (id: WizardStepId) =>
    t(`commandStation:admin.wizard.steps.${id}.label`);

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Box>
          <Link
            component={RouterLink}
            to="/admin/command-stations"
            underline="hover"
            sx={{
              display: "inline-flex",
              alignItems: "center",
              gap: 0.5,
              mb: 1,
            }}
          >
            <ArrowBackIcon fontSize="small" />
            {t("commandStation:admin.wizard.back")}
          </Link>
          <Typography variant="h4" component="h1" gutterBottom>
            {t("commandStation:admin.wizard.title")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("commandStation:admin.wizard.lead")}
          </Typography>
        </Box>

        <Stepper activeStep={stepIndex} alternativeLabel>
          {steps.map((id) => (
            <Step key={id}>
              <StepLabel>{stepLabel(id)}</StepLabel>
            </Step>
          ))}
        </Stepper>

        {actionError && <Alert severity="error">{actionError}</Alert>}

        <Paper variant="outlined" sx={{ p: { xs: 2, sm: 3 } }}>
          <Stack spacing={2}>
            <Typography variant="h6">
              {t(`commandStation:admin.wizard.steps.${activeStep}.title`)}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {t(`commandStation:admin.wizard.steps.${activeStep}.instruction`)}
            </Typography>
            {activeStep === "layout" && (
              <Alert severity="warning">
                {t("commandStation:admin.wizard.steps.layout.warning")}
              </Alert>
            )}

            {activeStep === "select" && (
              <>
                <Stack direction="row" spacing={2} alignItems="center" flexWrap="wrap">
                  <Button
                    variant="outlined"
                    onClick={rescan}
                    disabled={scanning}
                  >
                    {t("commandStation:admin.wizard.rescan")}
                  </Button>
                  {scanning && (
                    <>
                      <Button
                        variant="outlined"
                        color="inherit"
                        onClick={cancelScan}
                      >
                        {t("commandStation:admin.wizard.stopScan")}
                      </Button>
                      <Stack direction="row" spacing={1} alignItems="center">
                        <CircularProgress size={20} />
                        <Typography variant="body2" color="text.secondary">
                          {t("commandStation:admin.wizard.scanning")}
                        </Typography>
                      </Stack>
                    </>
                  )}
                </Stack>

                {scanError && (
                  <Alert severity="error" sx={{ whiteSpace: "pre-wrap" }}>
                    {scanError}
                  </Alert>
                )}

                {selected && isSerialAutodetectUri(selected.uri) && (
                  <Alert severity="warning">
                    {t("commandStation:admin.wizard.autodetectWarning")}
                  </Alert>
                )}

                {!scanning && rows.length === 0 && !scanError ? (
                  <Typography variant="body2" color="text.secondary">
                    {t("commandStation:admin.wizard.empty")}
                  </Typography>
                ) : null}

                {(rows.length > 0 || scanning) && (
                  <TableContainer>
                    <Table size="small">
                      <TableHead>
                        <TableRow>
                          <TableCell>
                            {t("commandStation:admin.wizard.columns.name")}
                          </TableCell>
                          <TableCell>
                            {t("commandStation:admin.wizard.columns.kind")}
                          </TableCell>
                          <TableCell>
                            {t("commandStation:admin.wizard.columns.uri")}
                          </TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {rows.map((row) => {
                          const kind = kindFromConnectionUri(row.uri);
                          const isSelected = selected?.uri === row.uri;
                          return (
                            <TableRow
                              key={row.uri}
                              hover
                              selected={isSelected}
                              onClick={() => selectRow(row)}
                              sx={{ cursor: "pointer" }}
                            >
                              <TableCell>{row.name}</TableCell>
                              <TableCell>
                                {t(`commandStation:admin.kind.${kind}`)}
                              </TableCell>
                              <TableCell>
                                <Typography
                                  variant="body2"
                                  color="text.secondary"
                                  sx={{ fontFamily: "monospace" }}
                                >
                                  {row.uri}
                                </Typography>
                              </TableCell>
                            </TableRow>
                          );
                        })}
                      </TableBody>
                    </Table>
                  </TableContainer>
                )}

                {selected && (
                  <TextField
                    label={t("commandStation:admin.dialogs.fields.name")}
                    value={nameInput}
                    onChange={(e) => setNameInput(e.target.value)}
                    fullWidth
                    required
                  />
                )}
              </>
            )}

            {activeStep === "remotes" && (
              <>
                <FormControlLabel
                  control={
                    <Switch
                      checked={z21ServerEnabled}
                      onChange={(_, v) => setZ21ServerEnabled(v)}
                    />
                  }
                  label={t(
                    "commandStation:admin.dialogs.fields.z21ServerEnabled",
                  )}
                />
                <FormHelperText sx={{ mt: -1, ml: 4 }}>
                  {t("commandStation:admin.dialogs.fields.z21ServerEnabledHelp")}
                </FormHelperText>
                <FormControlLabel
                  control={
                    <Switch
                      checked={withrottleServerEnabled}
                      onChange={(_, v) => setWithrottleServerEnabled(v)}
                    />
                  }
                  label={t(
                    "commandStation:admin.dialogs.fields.withrottleServerEnabled",
                  )}
                />
                <FormHelperText sx={{ mt: -1, ml: 4 }}>
                  {t(
                    "commandStation:admin.dialogs.fields.withrottleServerEnabledHelp",
                  )}
                </FormHelperText>
              </>
            )}

            {activeStep === "bootstop" && (
              <>
                <Alert severity="warning">
                  {t("commandStation:admin.wizard.steps.bootstop.alert")}
                </Alert>
                <FormControlLabel
                  control={
                    <Switch
                      checked={bootStopEnabled}
                      onChange={(_, v) => setBootStopEnabled(v)}
                    />
                  }
                  label={t(
                    "commandStation:admin.dialogs.fields.bootStopEnabled",
                  )}
                />
              </>
            )}

            {activeStep === "slots" && (
              <FormControlLabel
                control={
                  <Switch
                    checked={allocatePhysicalSlots}
                    onChange={(_, v) => setAllocatePhysicalSlots(v)}
                  />
                }
                label={t(
                  "commandStation:admin.dialogs.fields.allocatePhysicalSlots",
                )}
              />
            )}

            {activeStep === "layout" && (
              <>
                <FormControlLabel
                  control={
                    <Switch
                      checked={attachToLayout}
                      onChange={(_, v) => setAttachToLayout(v)}
                      disabled={layoutId == null}
                    />
                  }
                  label={t("commandStation:admin.wizard.attachCurrentLayout", {
                    name: layoutName || "—",
                  })}
                />
                <Typography variant="body2" color="text.secondary">
                  {t("commandStation:admin.wizard.skipHint")}
                </Typography>
                {selected && (
                  <Alert severity="info">
                    {t("commandStation:admin.wizard.summary", {
                      name: nameInput.trim() || selected.name,
                      kind: t(
                        `commandStation:admin.kind.${kindFromConnectionUri(selected.uri)}`,
                      ),
                      uri: selected.uri,
                      remotes: [
                        z21ServerEnabled ? "Z21" : null,
                        withrottleServerEnabled ? "WiFred" : null,
                      ]
                        .filter(Boolean)
                        .join(", ") || "—",
                      bootStop: bootStopEnabled
                        ? t("commandStation:admin.wizard.yes")
                        : t("commandStation:admin.wizard.no"),
                    })}
                  </Alert>
                )}
              </>
            )}
          </Stack>
        </Paper>

        <Stack
          direction="row"
          spacing={2}
          justifyContent="space-between"
          flexWrap="wrap"
          useFlexGap
        >
          <Button
            onClick={() => navigate("/admin/command-stations")}
            disabled={submitting}
          >
            {t("commandStation:admin.wizard.cancel")}
          </Button>
          <Stack direction="row" spacing={2}>
            <Button
              onClick={goBack}
              disabled={stepIndex === 0 || submitting}
            >
              {t("commandStation:admin.wizard.backStep")}
            </Button>
            <Button
              variant="contained"
              onClick={goNext}
              disabled={!canNext() || submitting || scanning}
            >
              {stepIndex >= steps.length - 1
                ? t("commandStation:admin.wizard.finish")
                : t("commandStation:admin.wizard.next")}
            </Button>
          </Stack>
        </Stack>
      </Stack>
    </Container>
  );
}
