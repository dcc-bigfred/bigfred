import { useEffect, useMemo, useState } from "react";
import AutoFixHighIcon from "@mui/icons-material/AutoFixHigh";
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  MenuItem,
  Stack,
  Switch,
  TextField,
  Tooltip,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";
import {
  DCC_FUNCTION_NUMBERS,
  DEADMAN_SWITCH_OPTIONS,
  DEFAULT_DEADMAN_SWITCH_OPTION,
  DEFAULT_EMERGENCY_LIGHTS_FUNCTION,
  DEFAULT_RP1_FUNCTION,
  isVehicleEpoch,
  useCreateVehicle,
  useMyDCCPool,
  useUpdateVehicle,
  VEHICLE_EPOCHS,
  VEHICLE_KINDS,
  type DeadManSwitchOption,
  type Vehicle,
  type VehicleKind,
} from "../api/vehicles";
import {
  isBigFredNativeMobileApp,
  openModelPicker,
  type ModelPickPayload,
} from "../native/bigfredNativeApp";

interface Props {
  open: boolean;
  vehicle?: Vehicle | null;
  onClose: () => void;
}

function applyModelPick(
  payload: ModelPickPayload,
  setters: {
    setNumber: (v: string) => void;
    setCarrier: (v: string) => void;
    setAssignment: (v: string) => void;
    setRevisionDate: (v: string) => void;
    setEpoch: (v: string) => void;
  },
) {
  const number = payload.vehicleNumber?.trim();
  if (number) setters.setNumber(number);

  const carrier = payload.carrier?.trim();
  if (carrier) setters.setCarrier(carrier);

  const assignment = payload.assignment?.trim();
  if (assignment) setters.setAssignment(assignment);

  const revision = payload.revisionDate?.trim();
  if (revision) setters.setRevisionDate(revision);

  const firstEpoch = payload.epochs?.find((e) => !!e?.trim())?.trim();
  if (firstEpoch && isVehicleEpoch(firstEpoch)) {
    setters.setEpoch(firstEpoch);
  }
}

// VehicleDialog handles the add-and-edit dialog for a single vehicle.
// It is a controlled component: the parent supplies `vehicle` for
// edit mode and `undefined`/`null` for create mode. The dialog
// surfaces the user's DCC pool as a helper hint so they understand
// why an out-of-pool address is rejected.
export default function VehicleDialog({ open, vehicle, onClose }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common"]);
  const isEdit = !!vehicle;
  const showFillFromCatalog = isBigFredNativeMobileApp();

  const [name, setName] = useState("");
  const [kind, setKind] = useState<VehicleKind>("loco");
  const [number, setNumber] = useState("");
  const [carrier, setCarrier] = useState("");
  const [assignment, setAssignment] = useState("");
  const [revisionDate, setRevisionDate] = useState("");
  const [epoch, setEpoch] = useState("");
  const [dccEnabled, setDccEnabled] = useState(true);
  const [dccAddress, setDccAddress] = useState<string>("");
  const [rp1Function, setRp1Function] = useState(DEFAULT_RP1_FUNCTION);
  const [emergencyLightsFunction, setEmergencyLightsFunction] = useState(
    DEFAULT_EMERGENCY_LIGHTS_FUNCTION,
  );
  const [deadManSwitchOption, setDeadManSwitchOption] =
    useState<DeadManSwitchOption>(DEFAULT_DEADMAN_SWITCH_OPTION);
  const [pickingModel, setPickingModel] = useState(false);

  const create = useCreateVehicle();
  const update = useUpdateVehicle();
  const pool = useMyDCCPool();

  useEffect(() => {
    if (!open) return;
    if (vehicle) {
      setName(vehicle.name);
      setKind(vehicle.kind);
      setNumber(vehicle.number);
      setCarrier(vehicle.carrier ?? "");
      setAssignment(vehicle.assignment ?? "");
      setRevisionDate(vehicle.revisionDate ?? "");
      setEpoch(vehicle.epoch ?? "");
      setDccEnabled(vehicle.dccAddress != null);
      setDccAddress(vehicle.dccAddress != null ? String(vehicle.dccAddress) : "");
      setRp1Function(vehicle.rp1Function ?? DEFAULT_RP1_FUNCTION);
      setEmergencyLightsFunction(
        vehicle.emergencyLightsFunction ?? DEFAULT_EMERGENCY_LIGHTS_FUNCTION,
      );
      setDeadManSwitchOption(
        vehicle.deadManSwitchOption ?? DEFAULT_DEADMAN_SWITCH_OPTION,
      );
    } else {
      setName("");
      setKind("loco");
      setNumber("");
      setCarrier("");
      setAssignment("");
      setRevisionDate("");
      setEpoch("");
      setDccEnabled(true);
      setDccAddress("");
      setRp1Function(DEFAULT_RP1_FUNCTION);
      setEmergencyLightsFunction(DEFAULT_EMERGENCY_LIGHTS_FUNCTION);
      setDeadManSwitchOption(DEFAULT_DEADMAN_SWITCH_OPTION);
    }
    create.reset();
    update.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, vehicle?.id]);

  const poolHint = useMemo(() => {
    if (!pool.data || pool.data.length === 0) {
      return t("vehicle:dialog.fields.poolEmpty");
    }
    const ranges = pool.data
      .map((r) => (r.from === r.to ? String(r.from) : `${r.from}..${r.to}`))
      .join(", ");
    return t("vehicle:dialog.fields.poolHint", { ranges });
  }, [pool.data, t]);

  const onFillFromCatalog = async () => {
    setPickingModel(true);
    try {
      const payload = await openModelPicker();
      if (!payload) return;
      applyModelPick(payload, {
        setNumber,
        setCarrier,
        setAssignment,
        setRevisionDate,
        setEpoch,
      });
    } finally {
      setPickingModel(false);
    }
  };

  const onSubmit = () => {
    const parsedAddr = dccEnabled ? Number(dccAddress) : null;
    const dccValue =
      dccEnabled && Number.isFinite(parsedAddr) && parsedAddr! > 0
        ? Math.trunc(parsedAddr!)
        : null;
    const catalogMeta = {
      carrier,
      assignment,
      revisionDate: revisionDate.trim() ? revisionDate.trim() : null,
      epoch,
    };
    if (isEdit && vehicle) {
      update.mutate({
        id: vehicle.id,
        name,
        kind,
        number,
        dccAddress: dccEnabled ? dccValue : null,
        rp1Function,
        emergencyLightsFunction,
        deadManSwitchOption,
        ...catalogMeta,
      });
    } else {
      create.mutate({
        name,
        kind,
        number,
        dccAddress: dccEnabled ? dccValue : null,
        rp1Function,
        emergencyLightsFunction,
        deadManSwitchOption,
        ...catalogMeta,
      });
    }
  };

  // Close the dialog once the in-flight mutation succeeds. We watch
  // the mutation status instead of awaiting in the handler so the
  // surrounding form stays in sync with TanStack Query's optimistic
  // invalidations.
  useEffect(() => {
    if (create.isSuccess || update.isSuccess) {
      onClose();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [create.isSuccess, update.isSuccess]);

  const errorMessage = (() => {
    const err = create.error ?? update.error;
    if (!err) return null;
    if (err instanceof ApiError) {
      const key = `errors:${err.code}` as const;
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  })();

  const submitting = create.isPending || update.isPending;
  const canSubmit = name.trim().length > 0 && !submitting;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
          pr: showFillFromCatalog ? 1 : undefined,
        }}
      >
        <Box component="span" sx={{ flex: 1, minWidth: 0 }}>
          {isEdit
            ? t("vehicle:dialog.edit.title")
            : t("vehicle:dialog.create.title")}
        </Box>
        {showFillFromCatalog && (
          <Tooltip title={t("vehicle:dialog.fillFromCatalog")}>
            <span>
              <IconButton
                edge="end"
                onClick={() => void onFillFromCatalog()}
                disabled={pickingModel || submitting}
                aria-label={t("vehicle:dialog.fillFromCatalog")}
              >
                <AutoFixHighIcon />
              </IconButton>
            </span>
          </Tooltip>
        )}
      </DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <TextField
            label={t("vehicle:dialog.fields.name")}
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
            fullWidth
            required
          />
          <TextField
            select
            label={t("vehicle:dialog.fields.kind")}
            value={kind}
            onChange={(e) => setKind(e.target.value as VehicleKind)}
            fullWidth
            required
          >
            {VEHICLE_KINDS.map((k) => (
              <MenuItem key={k} value={k}>
                {t(`vehicle:kind.${k}` as const)}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            label={t("vehicle:dialog.fields.number")}
            value={number}
            onChange={(e) => setNumber(e.target.value)}
            fullWidth
          />
          <TextField
            label={t("vehicle:dialog.fields.carrier")}
            value={carrier}
            onChange={(e) => setCarrier(e.target.value)}
            fullWidth
          />
          <TextField
            label={t("vehicle:dialog.fields.assignment")}
            value={assignment}
            onChange={(e) => setAssignment(e.target.value)}
            fullWidth
          />
          <TextField
            label={t("vehicle:dialog.fields.revisionDate")}
            type="date"
            value={revisionDate}
            onChange={(e) => setRevisionDate(e.target.value)}
            InputLabelProps={{ shrink: true }}
            fullWidth
          />
          <TextField
            select
            label={t("vehicle:dialog.fields.epoch")}
            value={epoch}
            onChange={(e) => setEpoch(e.target.value)}
            fullWidth
          >
            <MenuItem value="">
              {t("vehicle:dialog.fields.epochNone")}
            </MenuItem>
            {VEHICLE_EPOCHS.map((e) => (
              <MenuItem key={e} value={e}>
                {e}
              </MenuItem>
            ))}
          </TextField>
          <FormControlLabel
            control={
              <Switch
                checked={dccEnabled}
                onChange={(e) => setDccEnabled(e.target.checked)}
              />
            }
            label={t("vehicle:dialog.fields.dccAddress")}
          />
          <TextField
            label={t("vehicle:dialog.fields.dccAddress")}
            type="number"
            inputProps={{ min: 1, max: 9999 }}
            value={dccAddress}
            onChange={(e) => setDccAddress(e.target.value)}
            disabled={!dccEnabled}
            helperText={dccEnabled ? poolHint : t("vehicle:dialog.fields.dccAddressHelp")}
            fullWidth
          />
          <TextField
            select
            label={t("vehicle:dialog.fields.rp1Function")}
            value={rp1Function}
            onChange={(e) => setRp1Function(Number(e.target.value))}
            fullWidth
          >
            {DCC_FUNCTION_NUMBERS.map((fn) => (
              <MenuItem key={fn} value={fn}>
                {t("vehicle:dialog.fields.dccFunction", { fn })}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            label={t("vehicle:dialog.fields.emergencyLightsFunction")}
            value={emergencyLightsFunction}
            onChange={(e) => setEmergencyLightsFunction(Number(e.target.value))}
            fullWidth
          >
            {DCC_FUNCTION_NUMBERS.map((fn) => (
              <MenuItem key={fn} value={fn}>
                {t("vehicle:dialog.fields.dccFunction", { fn })}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            label={t("vehicle:dialog.fields.deadManSwitchOption")}
            value={deadManSwitchOption}
            onChange={(e) =>
              setDeadManSwitchOption(e.target.value as DeadManSwitchOption)
            }
            fullWidth
          >
            {DEADMAN_SWITCH_OPTIONS.map((opt) => (
              <MenuItem key={opt} value={opt}>
                {t(`vehicle:deadManSwitch.${opt}` as const)}
              </MenuItem>
            ))}
          </TextField>

          {errorMessage && <Alert severity="error">{errorMessage}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t("vehicle:dialog.cancel")}</Button>
        <Button onClick={onSubmit} disabled={!canSubmit} variant="contained">
          {isEdit
            ? t("vehicle:dialog.submitEdit")
            : t("vehicle:dialog.submitCreate")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
