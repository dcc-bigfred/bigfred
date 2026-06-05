import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  MenuItem,
  Stack,
  Switch,
  TextField,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";
import {
  DCC_FUNCTION_NUMBERS,
  DEADMAN_SWITCH_OPTIONS,
  DEFAULT_DEADMAN_SWITCH_OPTION,
  DEFAULT_EMERGENCY_LIGHTS_FUNCTION,
  DEFAULT_RP1_FUNCTION,
  useCreateVehicle,
  useMyDCCPool,
  useUpdateVehicle,
  VEHICLE_KINDS,
  type DeadManSwitchOption,
  type Vehicle,
  type VehicleKind,
} from "../api/vehicles";

interface Props {
  open: boolean;
  vehicle?: Vehicle | null;
  onClose: () => void;
}

// VehicleDialog handles the add-and-edit dialog for a single vehicle.
// It is a controlled component: the parent supplies `vehicle` for
// edit mode and `undefined`/`null` for create mode. The dialog
// surfaces the user's DCC pool as a helper hint so they understand
// why an out-of-pool address is rejected.
export default function VehicleDialog({ open, vehicle, onClose }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common"]);
  const isEdit = !!vehicle;

  const [name, setName] = useState("");
  const [kind, setKind] = useState<VehicleKind>("loco");
  const [number, setNumber] = useState("");
  const [dccEnabled, setDccEnabled] = useState(true);
  const [dccAddress, setDccAddress] = useState<string>("");
  const [rp1Function, setRp1Function] = useState(DEFAULT_RP1_FUNCTION);
  const [emergencyLightsFunction, setEmergencyLightsFunction] = useState(
    DEFAULT_EMERGENCY_LIGHTS_FUNCTION,
  );
  const [deadManSwitchOption, setDeadManSwitchOption] =
    useState<DeadManSwitchOption>(DEFAULT_DEADMAN_SWITCH_OPTION);

  const create = useCreateVehicle();
  const update = useUpdateVehicle();
  const pool = useMyDCCPool();

  useEffect(() => {
    if (!open) return;
    if (vehicle) {
      setName(vehicle.name);
      setKind(vehicle.kind);
      setNumber(vehicle.number);
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

  const onSubmit = () => {
    const parsedAddr = dccEnabled ? Number(dccAddress) : null;
    const dccValue =
      dccEnabled && Number.isFinite(parsedAddr) && parsedAddr! > 0
        ? Math.trunc(parsedAddr!)
        : null;
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
      <DialogTitle>
        {isEdit ? t("vehicle:dialog.edit.title") : t("vehicle:dialog.create.title")}
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
