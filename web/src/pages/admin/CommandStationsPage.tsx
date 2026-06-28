import { useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControl,
  FormControlLabel,
  IconButton,
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
  Tooltip,
  Typography,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import EditIcon from "@mui/icons-material/Edit";
import DeleteIcon from "@mui/icons-material/Delete";
import { useTranslation } from "react-i18next";

import { ApiError } from "../../api/client";
import {
  COMMAND_STATION_KINDS,
  COMMAND_STATION_SPEED_STEPS,
  useCommandStationsCatalogue,
  useCreateCommandStation,
  useDeleteCommandStation,
  useUpdateCommandStation,
  DEFAULT_COMMAND_STATION_DEADMAN_SECS,
  DEFAULT_COMMAND_STATION_HEARTBEAT_SECS,
  DEFAULT_COMMAND_STATION_POLL_INTERVAL_MS,
  DEFAULT_COMMAND_STATION_SPEED_STEPS,
  type CommandStation,
  type CommandStationKind,
} from "../../api/command_stations";

export default function CommandStationsPage() {
  const { t } = useTranslation(["commandStation", "common", "errors"]);
  const list = useCommandStationsCatalogue();
  const create = useCreateCommandStation();
  const update = useUpdateCommandStation();
  const remove = useDeleteCommandStation();

  type DialogState =
    | { kind: "create" }
    | { kind: "edit"; target: CommandStation }
    | { kind: "delete"; target: CommandStation }
    | null;

  const [dialog, setDialog] = useState<DialogState>(null);
  const [nameInput, setNameInput] = useState("");
  const [kindInput, setKindInput] = useState<CommandStationKind>("z21");
  const [uriInput, setUriInput] = useState("");
  const [speedStepsInput, setSpeedStepsInput] = useState<number>(
    DEFAULT_COMMAND_STATION_SPEED_STEPS,
  );
  const [heartbeatSecsInput, setHeartbeatSecsInput] = useState<number>(
    DEFAULT_COMMAND_STATION_HEARTBEAT_SECS,
  );
  const [deadmanSecsInput, setDeadmanSecsInput] = useState<number>(
    DEFAULT_COMMAND_STATION_DEADMAN_SECS,
  );
  const [pollIntervalMsInput, setPollIntervalMsInput] = useState<number>(
    DEFAULT_COMMAND_STATION_POLL_INTERVAL_MS,
  );
  const [z21ServerEnabledInput, setZ21ServerEnabledInput] = useState(false);
  const [z21IpStickinessInput, setZ21IpStickinessInput] = useState(false);
  const [withrottleServerEnabledInput, setWithrottleServerEnabledInput] =
    useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const closeDialog = () => {
    setDialog(null);
    setNameInput("");
    setKindInput("z21");
    setUriInput("");
    setSpeedStepsInput(DEFAULT_COMMAND_STATION_SPEED_STEPS);
    setHeartbeatSecsInput(DEFAULT_COMMAND_STATION_HEARTBEAT_SECS);
    setDeadmanSecsInput(DEFAULT_COMMAND_STATION_DEADMAN_SECS);
    setPollIntervalMsInput(DEFAULT_COMMAND_STATION_POLL_INTERVAL_MS);
    setZ21ServerEnabledInput(false);
    setZ21IpStickinessInput(false);
    setWithrottleServerEnabledInput(false);
    setActionError(null);
    create.reset();
    update.reset();
    remove.reset();
  };

  const translateError = (err: unknown): string => {
    if (err instanceof ApiError) {
      const localised = t(`errors:${err.code}` as const, { defaultValue: "" });
      if (localised) return localised;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  };

  const openCreate = () => {
    setDialog({ kind: "create" });
    setNameInput("");
    setKindInput("z21");
    setUriInput("");
    setSpeedStepsInput(DEFAULT_COMMAND_STATION_SPEED_STEPS);
    setHeartbeatSecsInput(DEFAULT_COMMAND_STATION_HEARTBEAT_SECS);
    setDeadmanSecsInput(DEFAULT_COMMAND_STATION_DEADMAN_SECS);
    setPollIntervalMsInput(DEFAULT_COMMAND_STATION_POLL_INTERVAL_MS);
    setZ21ServerEnabledInput(false);
    setZ21IpStickinessInput(false);
    setWithrottleServerEnabledInput(false);
    setActionError(null);
  };

  const openEdit = (target: CommandStation) => {
    setDialog({ kind: "edit", target });
    setNameInput(target.name);
    setKindInput(target.kind);
    setUriInput(target.connectionUri);
    setSpeedStepsInput(target.speedSteps);
    setHeartbeatSecsInput(target.heartbeatSecs);
    setDeadmanSecsInput(target.deadmanSecs);
    setPollIntervalMsInput(target.pollIntervalMs);
    setZ21ServerEnabledInput(target.z21ServerEnabled);
    setZ21IpStickinessInput(target.z21IpStickiness);
    setWithrottleServerEnabledInput(target.withrottleServerEnabled);
    setActionError(null);
  };

  const openDelete = (target: CommandStation) => {
    setDialog({ kind: "delete", target });
    setActionError(null);
  };

  const submitDialog = async () => {
    if (!dialog) return;
    try {
      const body = {
        name: nameInput.trim(),
        kind: kindInput,
        connectionUri: uriInput.trim(),
        speedSteps: speedStepsInput,
        heartbeatSecs: heartbeatSecsInput,
        deadmanSecs: deadmanSecsInput,
        pollIntervalMs: pollIntervalMsInput,
        z21ServerEnabled: z21ServerEnabledInput,
        z21IpStickiness: z21ServerEnabledInput ? z21IpStickinessInput : false,
        withrottleServerEnabled: withrottleServerEnabledInput,
      };
      if (dialog.kind === "create") {
        await create.mutateAsync(body);
      } else if (dialog.kind === "edit") {
        await update.mutateAsync({ id: dialog.target.id, ...body });
      } else if (dialog.kind === "delete") {
        await remove.mutateAsync(dialog.target.id);
      }
      closeDialog();
    } catch (err) {
      setActionError(translateError(err));
    }
  };

  const rows = useMemo(() => list.data ?? [], [list.data]);
  const submitting = create.isPending || update.isPending || remove.isPending;

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Stack
          direction={{ xs: "column", sm: "row" }}
          alignItems={{ xs: "flex-start", sm: "center" }}
          justifyContent="space-between"
          spacing={2}
        >
          <Box>
            <Typography variant="h4" component="h1" gutterBottom>
              {t("commandStation:admin.title")}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {t("commandStation:admin.subtitle")}
            </Typography>
          </Box>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={openCreate}
            disabled={submitting}
          >
            {t("commandStation:admin.actions.create")}
          </Button>
        </Stack>

        {list.isError && (
          <Alert severity="error">{translateError(list.error)}</Alert>
        )}

        <Paper variant="outlined">
          {list.isLoading ? (
            <Box sx={{ p: 4, display: "flex", justifyContent: "center" }}>
              <CircularProgress />
            </Box>
          ) : rows.length === 0 ? (
            <Box sx={{ p: 4 }}>
              <Typography variant="body2" color="text.secondary">
                {t("commandStation:admin.empty")}
              </Typography>
            </Box>
          ) : (
            <TableContainer>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>{t("commandStation:admin.columns.name")}</TableCell>
                    <TableCell>{t("commandStation:admin.columns.kind")}</TableCell>
                    <TableCell>{t("commandStation:admin.columns.connection")}</TableCell>
                    <TableCell>{t("commandStation:admin.columns.speedSteps")}</TableCell>
                    <TableCell>{t("commandStation:admin.columns.timing")}</TableCell>
                    <TableCell>{t("commandStation:admin.columns.z21Server")}</TableCell>
                    <TableCell align="right">
                      {t("commandStation:admin.columns.actions")}
                    </TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {rows.map((row) => (
                    <TableRow key={row.id} hover>
                      <TableCell>
                        <Typography variant="body2" fontWeight={500}>
                          {row.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        {t(`commandStation:admin.kind.${row.kind}`)}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {row.connectionUri || "—"}
                        </Typography>
                      </TableCell>
                      <TableCell>{row.speedSteps}</TableCell>
                      <TableCell>
                        {t("commandStation:admin.timingSummary", {
                          heartbeat: row.heartbeatSecs,
                          deadman: row.deadmanSecs,
                          poll: row.pollIntervalMs,
                        })}
                      </TableCell>
                      <TableCell>
                        {row.z21ServerEnabled ? (
                          <Chip
                            size="small"
                            color="success"
                            label={t("commandStation:admin.z21ServerOn")}
                          />
                        ) : (
                          "—"
                        )}
                      </TableCell>
                      <TableCell align="right">
                        <Stack
                          direction="row"
                          spacing={0.5}
                          justifyContent="flex-end"
                        >
                          <Tooltip title={t("commandStation:admin.actions.edit")}>
                            <IconButton
                              size="small"
                              onClick={() => openEdit(row)}
                              disabled={submitting}
                              aria-label={t("commandStation:admin.actions.edit")}
                            >
                              <EditIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title={t("commandStation:admin.actions.delete")}>
                            <IconButton
                              size="small"
                              color="error"
                              onClick={() => openDelete(row)}
                              disabled={submitting}
                              aria-label={t("commandStation:admin.actions.delete")}
                            >
                              <DeleteIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </Paper>
      </Stack>

      <Dialog
        open={dialog?.kind === "create" || dialog?.kind === "edit"}
        onClose={closeDialog}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>
          {dialog?.kind === "edit"
            ? t("commandStation:admin.dialogs.edit.title")
            : t("commandStation:admin.dialogs.create.title")}
        </DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label={t("commandStation:admin.dialogs.fields.name")}
              value={nameInput}
              onChange={(e) => setNameInput(e.target.value)}
              autoFocus
              fullWidth
              required
            />
            <FormControl fullWidth>
              <InputLabel>{t("commandStation:admin.dialogs.fields.kind")}</InputLabel>
              <Select
                value={kindInput}
                label={t("commandStation:admin.dialogs.fields.kind")}
                onChange={(e) =>
                  setKindInput(e.target.value as CommandStationKind)
                }
              >
                {COMMAND_STATION_KINDS.map((kind) => (
                  <MenuItem key={kind} value={kind}>
                    {t(`commandStation:admin.kind.${kind}`)}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField
              label={t("commandStation:admin.dialogs.fields.connectionUri")}
              value={uriInput}
              onChange={(e) => setUriInput(e.target.value)}
              helperText={t(
                "commandStation:admin.dialogs.fields.connectionUriHelp",
              )}
              fullWidth
            />
            <FormControl fullWidth>
              <InputLabel>
                {t("commandStation:admin.dialogs.fields.speedSteps")}
              </InputLabel>
              <Select
                value={String(speedStepsInput)}
                label={t("commandStation:admin.dialogs.fields.speedSteps")}
                onChange={(e) => setSpeedStepsInput(Number(e.target.value))}
              >
                {COMMAND_STATION_SPEED_STEPS.map((steps) => (
                  <MenuItem key={steps} value={String(steps)}>
                    {steps}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField
              label={t("commandStation:admin.dialogs.fields.pollIntervalMs")}
              type="number"
              value={pollIntervalMsInput}
              onChange={(e) =>
                setPollIntervalMsInput(Number(e.target.value))
              }
              helperText={t(
                "commandStation:admin.dialogs.fields.pollIntervalMsHelp",
              )}
              inputProps={{ min: 0, max: 60000, step: 50 }}
              fullWidth
            />
            <TextField
              label={t("commandStation:admin.dialogs.fields.heartbeatSecs")}
              type="number"
              value={heartbeatSecsInput}
              onChange={(e) =>
                setHeartbeatSecsInput(Number(e.target.value))
              }
              helperText={t(
                "commandStation:admin.dialogs.fields.heartbeatSecsHelp",
              )}
              inputProps={{ min: 1, max: 60, step: 0.5 }}
              fullWidth
              required
            />
            <TextField
              label={t("commandStation:admin.dialogs.fields.deadmanSecs")}
              type="number"
              value={deadmanSecsInput}
              onChange={(e) => setDeadmanSecsInput(Number(e.target.value))}
              helperText={t(
                "commandStation:admin.dialogs.fields.deadmanSecsHelp",
              )}
              inputProps={{ min: 3, max: 120, step: 0.5 }}
              fullWidth
              required
            />
            <FormControlLabel
              control={
                <Switch
                  checked={z21ServerEnabledInput}
                  onChange={(e) => {
                    const on = e.target.checked;
                    setZ21ServerEnabledInput(on);
                    if (!on) {
                      setZ21IpStickinessInput(false);
                    }
                  }}
                />
              }
              label={t("commandStation:admin.dialogs.fields.z21ServerEnabled")}
            />
            <Typography variant="body2" color="text.secondary">
              {t("commandStation:admin.dialogs.fields.z21ServerEnabledHelp")}
            </Typography>
            {z21ServerEnabledInput && (
              <>
                <FormControlLabel
                  control={
                    <Switch
                      checked={z21IpStickinessInput}
                      onChange={(e) =>
                        setZ21IpStickinessInput(e.target.checked)
                      }
                    />
                  }
                  label={t(
                    "commandStation:admin.dialogs.fields.z21IpStickiness",
                  )}
                />
                <Typography variant="body2" color="text.secondary">
                  {t(
                    "commandStation:admin.dialogs.fields.z21IpStickinessHelp",
                  )}
                </Typography>
              </>
            )}
            {dialog?.kind === "edit" &&
              dialog.target.z21ServerEnabled &&
              !z21ServerEnabledInput && (
                <Alert severity="warning">
                  {t("commandStation:admin.dialogs.fields.z21ServerDisableWarn")}
                </Alert>
              )}
            <FormControlLabel
              control={
                <Switch
                  checked={withrottleServerEnabledInput}
                  onChange={(e) =>
                    setWithrottleServerEnabledInput(e.target.checked)
                  }
                />
              }
              label={t(
                "commandStation:admin.dialogs.fields.withrottleServerEnabled",
              )}
            />
            <Typography variant="body2" color="text.secondary">
              {t(
                "commandStation:admin.dialogs.fields.withrottleServerEnabledHelp",
              )}
            </Typography>
            {dialog?.kind === "edit" &&
              dialog.target.withrottleServerEnabled &&
              !withrottleServerEnabledInput && (
                <Alert severity="warning">
                  {t(
                    "commandStation:admin.dialogs.fields.withrottleServerDisableWarn",
                  )}
                </Alert>
              )}
            {actionError && <Alert severity="error">{actionError}</Alert>}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDialog} disabled={submitting}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            variant="contained"
            onClick={submitDialog}
            disabled={
              submitting ||
              nameInput.trim() === "" ||
              heartbeatSecsInput <= 0 ||
              deadmanSecsInput <= heartbeatSecsInput ||
              pollIntervalMsInput < 0 ||
              pollIntervalMsInput > 60000
            }
          >
            {dialog?.kind === "edit"
              ? t("common:actions.save")
              : t("common:actions.create")}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={dialog?.kind === "delete"}
        onClose={closeDialog}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>
          {dialog?.kind === "delete" &&
            t("commandStation:admin.dialogs.delete.title", {
              name: dialog.target.name,
            })}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("commandStation:admin.dialogs.delete.message")}
          </DialogContentText>
          {actionError && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {actionError}
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDialog} disabled={submitting}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            variant="contained"
            color="error"
            onClick={submitDialog}
            disabled={submitting}
          >
            {t("common:actions.delete")}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
