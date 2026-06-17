import { useEffect, useMemo, useState } from "react";
import {
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

const MIN_MULTIPLIER = 0.05;
const MAX_MULTIPLIER = 4.0;
const STEP = 0.05;

export const START_DELAY_MS_OPTIONS = Array.from(
  { length: (1000 - 50) / 50 + 1 },
  (_, i) => 50 + i * 50,
);

export const RAMP_MS_OPTIONS = Array.from(
  { length: 5000 / 500 + 1 },
  (_, i) => i * 500,
);

export const RAMP_MAX_STEPS_OPTIONS = Array.from(
  { length: 10 },
  (_, i) => i + 1,
);

export interface TrainMemberSettings {
  speedMultiplier: number;
  excludeFromSpeed: boolean;
  startDelayMs: number;
  accelRampMs: number;
  accelRampMaxSteps: number;
  brakeRampMs: number;
  brakeRampMaxSteps: number;
}

export interface TrainMemberSettingsDialogProps {
  open: boolean;
  memberName: string;
  isLeading: boolean;
  initialSettings: TrainMemberSettings;
  saving?: boolean;
  onClose: () => void;
  onSave: (settings: TrainMemberSettings) => void;
}

function MemberRampControls({
  startDelayMs,
  accelRampMs,
  accelRampMaxSteps,
  brakeRampMs,
  brakeRampMaxSteps,
  disabled,
  onStartDelayMsChange,
  onAccelRampMsChange,
  onAccelRampMaxStepsChange,
  onBrakeRampMsChange,
  onBrakeRampMaxStepsChange,
}: {
  startDelayMs: number;
  accelRampMs: number;
  accelRampMaxSteps: number;
  brakeRampMs: number;
  brakeRampMaxSteps: number;
  disabled: boolean;
  onStartDelayMsChange: (ms: number) => void;
  onAccelRampMsChange: (ms: number) => void;
  onAccelRampMaxStepsChange: (steps: number) => void;
  onBrakeRampMsChange: (ms: number) => void;
  onBrakeRampMaxStepsChange: (steps: number) => void;
}) {
  const { t } = useTranslation("throttle");
  const startDelayOptions = useMemo(() => [0, ...START_DELAY_MS_OPTIONS], []);
  const rampDurationOptions = useMemo(() => RAMP_MS_OPTIONS, []);

  return (
    <>
      <FormControl fullWidth disabled={disabled} sx={{ mb: 2 }}>
        <InputLabel id="start-delay-label">
          {t("train.memberSettings.startDelay")}
        </InputLabel>
        <Select
          labelId="start-delay-label"
          label={t("train.memberSettings.startDelay")}
          value={String(startDelayMs)}
          onChange={(ev) => onStartDelayMsChange(Number(ev.target.value))}
        >
          {startDelayOptions.map((ms) => (
            <MenuItem key={ms} value={String(ms)}>
              {ms === 0
                ? t("train.memberSettings.startDelayNone")
                : t("train.memberSettings.startDelayMs", { ms })}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <FormControl fullWidth disabled={disabled} sx={{ mb: 2 }}>
        <InputLabel id="accel-ramp-duration-label">
          {t("train.memberSettings.accelRampDuration")}
        </InputLabel>
        <Select
          labelId="accel-ramp-duration-label"
          label={t("train.memberSettings.accelRampDuration")}
          value={String(accelRampMs)}
          onChange={(ev) => onAccelRampMsChange(Number(ev.target.value))}
        >
          {rampDurationOptions.map((ms) => (
            <MenuItem key={ms} value={String(ms)}>
              {ms === 0
                ? t("train.memberSettings.accelRampOff")
                : t("train.memberSettings.accelRampSeconds", {
                    seconds: ms / 1000,
                  })}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <FormControl fullWidth disabled={disabled || accelRampMs === 0} sx={{ mb: 2 }}>
        <InputLabel id="accel-ramp-steps-label">
          {t("train.memberSettings.accelRampMaxSteps")}
        </InputLabel>
        <Select
          labelId="accel-ramp-steps-label"
          label={t("train.memberSettings.accelRampMaxSteps")}
          value={String(accelRampMaxSteps)}
          onChange={(ev) => onAccelRampMaxStepsChange(Number(ev.target.value))}
        >
          {RAMP_MAX_STEPS_OPTIONS.map((steps) => (
            <MenuItem key={steps} value={String(steps)}>
              {steps}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <FormControl fullWidth disabled={disabled} sx={{ mb: 2 }}>
        <InputLabel id="brake-ramp-duration-label">
          {t("train.memberSettings.brakeRampDuration")}
        </InputLabel>
        <Select
          labelId="brake-ramp-duration-label"
          label={t("train.memberSettings.brakeRampDuration")}
          value={String(brakeRampMs)}
          onChange={(ev) => onBrakeRampMsChange(Number(ev.target.value))}
        >
          {rampDurationOptions.map((ms) => (
            <MenuItem key={ms} value={String(ms)}>
              {ms === 0
                ? t("train.memberSettings.brakeRampOff")
                : t("train.memberSettings.brakeRampSeconds", {
                    seconds: ms / 1000,
                  })}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
      <FormControl fullWidth disabled={disabled || brakeRampMs === 0}>
        <InputLabel id="brake-ramp-steps-label">
          {t("train.memberSettings.brakeRampMaxSteps")}
        </InputLabel>
        <Select
          labelId="brake-ramp-steps-label"
          label={t("train.memberSettings.brakeRampMaxSteps")}
          value={String(brakeRampMaxSteps)}
          onChange={(ev) => onBrakeRampMaxStepsChange(Number(ev.target.value))}
        >
          {RAMP_MAX_STEPS_OPTIONS.map((steps) => (
            <MenuItem key={steps} value={String(steps)}>
              {steps}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    </>
  );
}

export default function TrainMemberSettingsDialog({
  open,
  memberName,
  isLeading,
  initialSettings,
  saving = false,
  onClose,
  onSave,
}: TrainMemberSettingsDialogProps) {
  const { t } = useTranslation(["throttle", "errors"]);
  const [multiplier, setMultiplier] = useState(
    String(initialSettings.speedMultiplier),
  );
  const [excludeFromSpeed, setExcludeFromSpeed] = useState(
    initialSettings.excludeFromSpeed,
  );
  const [startDelayMs, setStartDelayMs] = useState(initialSettings.startDelayMs);
  const [accelRampMs, setAccelRampMs] = useState(initialSettings.accelRampMs);
  const [accelRampMaxSteps, setAccelRampMaxSteps] = useState(
    initialSettings.accelRampMaxSteps,
  );
  const [brakeRampMs, setBrakeRampMs] = useState(initialSettings.brakeRampMs);
  const [brakeRampMaxSteps, setBrakeRampMaxSteps] = useState(
    initialSettings.brakeRampMaxSteps,
  );

  useEffect(() => {
    if (open) {
      setMultiplier(String(initialSettings.speedMultiplier));
      setExcludeFromSpeed(initialSettings.excludeFromSpeed);
      setStartDelayMs(initialSettings.startDelayMs);
      setAccelRampMs(initialSettings.accelRampMs);
      setAccelRampMaxSteps(initialSettings.accelRampMaxSteps);
      setBrakeRampMs(initialSettings.brakeRampMs);
      setBrakeRampMaxSteps(initialSettings.brakeRampMaxSteps);
    }
  }, [
    open,
    initialSettings.speedMultiplier,
    initialSettings.excludeFromSpeed,
    initialSettings.startDelayMs,
    initialSettings.accelRampMs,
    initialSettings.accelRampMaxSteps,
    initialSettings.brakeRampMs,
    initialSettings.brakeRampMaxSteps,
  ]);

  const parsed = Number(multiplier.replace(",", "."));
  const valid =
    Number.isFinite(parsed) && parsed >= MIN_MULTIPLIER && parsed <= MAX_MULTIPLIER;
  const rampsDisabled = !isLeading && excludeFromSpeed;

  const buildSettings = (): TrainMemberSettings => ({
    speedMultiplier: isLeading ? 1 : parsed,
    excludeFromSpeed: isLeading ? false : excludeFromSpeed,
    startDelayMs: rampsDisabled ? 0 : startDelayMs,
    accelRampMs: rampsDisabled ? 0 : accelRampMs,
    accelRampMaxSteps:
      rampsDisabled || accelRampMs === 0 ? 1 : accelRampMaxSteps,
    brakeRampMs: rampsDisabled ? 0 : brakeRampMs,
    brakeRampMaxSteps:
      rampsDisabled || brakeRampMs === 0 ? 1 : brakeRampMaxSteps,
  });

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>{t("throttle:train.memberSettings.title")}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {memberName}
        </Typography>
        {isLeading ? (
          <>
            <Typography variant="body2" sx={{ mb: 2 }}>
              {t("throttle:train.memberSettings.leadingMultiplierFixed")}
            </Typography>
            <MemberRampControls
              startDelayMs={startDelayMs}
              accelRampMs={accelRampMs}
              accelRampMaxSteps={accelRampMaxSteps}
              brakeRampMs={brakeRampMs}
              brakeRampMaxSteps={brakeRampMaxSteps}
              disabled={false}
              onStartDelayMsChange={setStartDelayMs}
              onAccelRampMsChange={setAccelRampMs}
              onAccelRampMaxStepsChange={setAccelRampMaxSteps}
              onBrakeRampMsChange={setBrakeRampMs}
              onBrakeRampMaxStepsChange={setBrakeRampMaxSteps}
            />
          </>
        ) : (
          <>
            <TextField
              fullWidth
              type="number"
              label={t("throttle:train.multiplier.field")}
              value={multiplier}
              onChange={(ev) => setMultiplier(ev.target.value)}
              inputProps={{ min: MIN_MULTIPLIER, max: MAX_MULTIPLIER, step: STEP }}
              helperText={t("throttle:train.multiplier.help")}
              sx={{ mb: 2 }}
              disabled={excludeFromSpeed}
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={excludeFromSpeed}
                  onChange={(ev) => setExcludeFromSpeed(ev.target.checked)}
                />
              }
              label={t("throttle:train.memberSettings.excludeFromSpeed")}
              sx={{ display: "block", mb: 2 }}
            />
            <MemberRampControls
              startDelayMs={startDelayMs}
              accelRampMs={accelRampMs}
              accelRampMaxSteps={accelRampMaxSteps}
              brakeRampMs={brakeRampMs}
              brakeRampMaxSteps={brakeRampMaxSteps}
              disabled={rampsDisabled}
              onStartDelayMsChange={setStartDelayMs}
              onAccelRampMsChange={setAccelRampMs}
              onAccelRampMaxStepsChange={setAccelRampMaxSteps}
              onBrakeRampMsChange={setBrakeRampMs}
              onBrakeRampMaxStepsChange={setBrakeRampMaxSteps}
            />
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t("throttle:train.multiplier.cancel")}</Button>
        <Button
          variant="contained"
          disabled={(isLeading ? false : !valid) || saving}
          onClick={() => onSave(buildSettings())}
        >
          {t("throttle:train.multiplier.save")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
