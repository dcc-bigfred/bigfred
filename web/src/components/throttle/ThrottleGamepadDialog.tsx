import { useCallback, useEffect, useRef, useState } from "react";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  Slider,
  Stack,
  Switch,
  TextField,
  Typography,
} from "@mui/material";
import CloseIcon from "@mui/icons-material/Close";
import SportsEsportsIcon from "@mui/icons-material/SportsEsports";
import { useTranslation } from "react-i18next";

import type { ThrottleCockpitFunction } from "./ThrottleCockpit";
import GamepadAxisVisualizer from "./GamepadAxisVisualizer";
import type { ConnectedGamepad } from "../../hooks/useGamepads";
import {
  axisToSpeed,
  finalizeIdleAxisRange,
  GAMEPAD_IDLE_LEARN_SECONDS,
  GAMEPAD_SPEED_SENSITIVITY_DIVISORS,
  GAMEPAD_SPEED_SENSITIVITY_MAX,
  MAX_SPEED_BUTTON_STEPS,
  MIN_SPEED_BUTTON_STEPS,
  DEFAULT_SPEED_BUTTON_STEPS,
  formatSpeedSensitivityDivisor,
  parseSpeedButtonSteps,
  speedButtonStepSize,
  hasIdleCalibration,
  isGamepadSetupComplete,
  type GamepadMapping,
  type GamepadSpeedSensitivity,
} from "../../hooks/gamepadMapping";

type SetupStep = "warning" | "detect" | "idle" | "settings";

type LearnTarget =
  | { kind: "axis" }
  | { kind: "fn"; num: number }
  | { kind: "reverse" }
  | { kind: "stop" }
  | { kind: "accelerate" }
  | { kind: "decelerate" };

interface ThrottleGamepadDialogProps {
  open: boolean;
  onClose: () => void;
  gamepads: ConnectedGamepad[];
  configuredFunctions: ThrottleCockpitFunction[];
  mapping: GamepadMapping;
  maxSpeed: number;
  onMappingChange: (mapping: GamepadMapping) => void;
  onConfirm: (mapping: GamepadMapping) => void;
}

function buttonLabel(index: number): string {
  return `B${index}`;
}

function resolveInitialStep(
  gamepads: ConnectedGamepad[],
  mapping: GamepadMapping,
): SetupStep {
  if (gamepads.length === 0) {
    return "detect";
  }
  if (mapping.enabled && hasIdleCalibration(mapping)) {
    return "settings";
  }
  return "idle";
}

export default function ThrottleGamepadDialog({
  open,
  onClose,
  gamepads,
  configuredFunctions,
  mapping,
  maxSpeed,
  onMappingChange,
  onConfirm,
}: ThrottleGamepadDialogProps) {
  const { t } = useTranslation("throttle");
  const [step, setStep] = useState<SetupStep>("warning");
  const [draft, setDraft] = useState(mapping);
  const [learn, setLearn] = useState<LearnTarget | null>(null);
  const [previewSpeed, setPreviewSpeed] = useState(0);
  const [previewAxis, setPreviewAxis] = useState(0);
  const [previewAxes, setPreviewAxes] = useState<number[]>([]);
  const [idleLearnMin, setIdleLearnMin] = useState<number | null>(null);
  const [idleLearnMax, setIdleLearnMax] = useState<number | null>(null);
  const [idleCollecting, setIdleCollecting] = useState(false);
  const [idleSecondsLeft, setIdleSecondsLeft] = useState(
    GAMEPAD_IDLE_LEARN_SECONDS,
  );
  const [idleError, setIdleError] = useState<string | null>(null);
  const learnBaselineRef = useRef<number[] | null>(null);
  const prevButtonsRef = useRef<boolean[]>([]);
  const idleMinRef = useRef<number | null>(null);
  const idleMaxRef = useRef<number | null>(null);
  const wasOpenRef = useRef(false);

  useEffect(() => {
    const justOpened = open && !wasOpenRef.current;
    wasOpenRef.current = open;

    if (!open || !justOpened) {
      return;
    }
    setDraft(mapping);
    setLearn(null);
    setIdleCollecting(false);
    setIdleSecondsLeft(GAMEPAD_IDLE_LEARN_SECONDS);
    setIdleError(null);
    idleMinRef.current = null;
    idleMaxRef.current = null;
    setIdleLearnMin(null);
    setIdleLearnMax(null);
    setStep(isGamepadSetupComplete(mapping) ? "settings" : "warning");
  }, [open, mapping]);

  useEffect(() => {
    if (!open || step !== "detect" || gamepads.length === 0) {
      return;
    }
    const pad = gamepads[0];
    setDraft((prev) => ({ ...prev, gamepadId: pad.id }));
    setStep(resolveInitialStep(gamepads, mapping));
  }, [open, step, gamepads, mapping]);

  const activePad =
    gamepads.find((gp) => gp.id === draft.gamepadId) ?? gamepads[0] ?? null;

  const updateDraft = useCallback((patch: Partial<GamepadMapping>) => {
    setDraft((prev) => ({ ...prev, ...patch }));
  }, []);

  const finishIdleLearning = useCallback(() => {
    const min = idleMinRef.current;
    const max = idleMaxRef.current;
    if (min == null || max == null) {
      return;
    }
    setDraft((prev) => ({
      ...prev,
      ...finalizeIdleAxisRange(min, max),
    }));
    setIdleCollecting(false);
    setIdleError(null);
    setIdleLearnMin(null);
    setIdleLearnMax(null);
    setStep("settings");
  }, []);

  const abortIdleLearning = useCallback((message: string) => {
    setIdleCollecting(false);
    setIdleError(message);
    setIdleSecondsLeft(GAMEPAD_IDLE_LEARN_SECONDS);
    idleMinRef.current = null;
    idleMaxRef.current = null;
    setIdleLearnMin(null);
    setIdleLearnMax(null);
  }, []);

  const startIdleLearning = useCallback(() => {
    setIdleError(null);
    setIdleCollecting(true);
    setIdleSecondsLeft(GAMEPAD_IDLE_LEARN_SECONDS);
    idleMinRef.current = null;
    idleMaxRef.current = null;
    setIdleLearnMin(null);
    setIdleLearnMax(null);
  }, []);

  useEffect(() => {
    if (!idleCollecting) {
      return;
    }

    const interval = window.setInterval(() => {
      setIdleSecondsLeft((prev) => Math.max(0, prev - 1));
    }, 1000);

    const timeout = window.setTimeout(() => {
      finishIdleLearning();
    }, GAMEPAD_IDLE_LEARN_SECONDS * 1000);

    return () => {
      window.clearInterval(interval);
      window.clearTimeout(timeout);
    };
  }, [idleCollecting, finishIdleLearning]);

  useEffect(() => {
    if (!open || activePad == null) {
      learnBaselineRef.current = null;
      prevButtonsRef.current = [];
      return;
    }

    let frame = 0;

    const tick = () => {
      const gp = navigator.getGamepads?.()[activePad.index];
      if (!gp?.connected) {
        frame = requestAnimationFrame(tick);
        return;
      }

      const axisValue = gp.axes[draft.speedAxis] ?? 0;
      setPreviewAxes([...gp.axes]);
      setPreviewAxis(axisValue);
      setPreviewSpeed(axisToSpeed(axisValue, maxSpeed, draft));

      if (step === "idle" && idleCollecting) {
        const anyButton = gp.buttons.some((b) => b.pressed);
        if (anyButton) {
          abortIdleLearning(t("gamepad.idleAbortedInput"));
        } else if (idleMinRef.current == null || idleMaxRef.current == null) {
          idleMinRef.current = axisValue;
          idleMaxRef.current = axisValue;
          setIdleLearnMin(axisValue);
          setIdleLearnMax(axisValue);
        } else {
          idleMinRef.current = Math.min(idleMinRef.current, axisValue);
          idleMaxRef.current = Math.max(idleMaxRef.current, axisValue);
          setIdleLearnMin(idleMinRef.current);
          setIdleLearnMax(idleMaxRef.current);
        }
      }

      if (step === "settings" && learn?.kind === "axis") {
        if (!learnBaselineRef.current) {
          learnBaselineRef.current = [...gp.axes];
        } else {
          const baseline = learnBaselineRef.current;
          let best = -1;
          let bestDelta = 0;
          for (let i = 0; i < gp.axes.length; i++) {
            const delta = Math.abs(gp.axes[i] - (baseline[i] ?? 0));
            if (delta > bestDelta) {
              bestDelta = delta;
              best = i;
            }
          }
          if (bestDelta > 0.35 && best >= 0) {
            updateDraft({ speedAxis: best });
            setLearn(null);
            learnBaselineRef.current = null;
          }
        }
      }

      if (step === "settings" && learn && learn.kind !== "axis") {
        const prev = prevButtonsRef.current;
        for (let i = 0; i < gp.buttons.length; i++) {
          const pressed = gp.buttons[i]?.pressed ?? false;
          if (pressed && !prev[i]) {
            if (learn.kind === "fn") {
              setDraft((current) => ({
                ...current,
                fnButtons: { ...current.fnButtons, [learn.num]: i },
              }));
            } else if (learn.kind === "reverse") {
              setDraft((current) => ({ ...current, reverseButton: i }));
            } else if (learn.kind === "stop") {
              setDraft((current) => ({ ...current, stopButton: i }));
            } else if (learn.kind === "accelerate") {
              setDraft((current) => ({ ...current, accelerateButton: i }));
            } else if (learn.kind === "decelerate") {
              setDraft((current) => ({ ...current, decelerateButton: i }));
            }
            setLearn(null);
            break;
          }
        }
      }

      prevButtonsRef.current = gp.buttons.map((b) => b.pressed);
      frame = requestAnimationFrame(tick);
    };

    frame = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(frame);
      learnBaselineRef.current = null;
    };
  }, [
    open,
    activePad,
    step,
    idleCollecting,
    learn,
    draft,
    maxSpeed,
    updateDraft,
    abortIdleLearning,
    t,
  ]);

  useEffect(() => {
    if (activePad && draft.gamepadId !== activePad.id) {
      updateDraft({ gamepadId: activePad.id });
    }
  }, [activePad, draft.gamepadId, updateDraft]);

  const handleConfirm = () => {
    const next: GamepadMapping = { ...draft, enabled: draft.enabled };
    if (!next.enabled) {
      next.idleAxisMin = undefined;
      next.idleAxisMax = undefined;
    }
    onMappingChange(next);
    onConfirm(next);
    onClose();
  };

  const learnHint = (() => {
    if (step !== "settings" || !learn) return null;
    if (learn.kind === "axis") return t("gamepad.learnAxisHint");
    return t("gamepad.learnButtonHint");
  })();

  const acknowledgeWarning = useCallback(() => {
    setStep(resolveInitialStep(gamepads, mapping));
  }, [gamepads, mapping]);

  const renderAxisVisualizer = (liveLearn: boolean) =>
    activePad != null ? (
      <GamepadAxisVisualizer
        title={t("gamepad.axisVisualizerTitle")}
        axes={previewAxes}
        speedAxis={draft.speedAxis}
        idleMin={draft.idleAxisMin}
        idleMax={draft.idleAxisMax}
        liveLearnMin={liveLearn ? idleLearnMin : null}
        liveLearnMax={liveLearn ? idleLearnMax : null}
      />
    ) : null;

  const renderWarningStep = () => (
    <Alert severity="warning" sx={{ whiteSpace: "pre-line" }}>
      {t("gamepad.safetyWarning")}
    </Alert>
  );

  const renderDetectStep = () => (
    <Stack spacing={2} alignItems="center" sx={{ py: 2 }}>
      <SportsEsportsIcon sx={{ fontSize: 48, opacity: 0.6 }} />
      <Typography align="center" color="text.secondary">
        {t("gamepad.detectHint")}
      </Typography>
    </Stack>
  );

  const renderIdleStep = () => (
    <Stack spacing={2.5}>
      <Box>
        <Typography variant="subtitle2" gutterBottom>
          {t("gamepad.connected")}
        </Typography>
        <Typography variant="body2">
          {activePad?.id ?? draft.gamepadId}
        </Typography>
      </Box>

      {renderAxisVisualizer(idleCollecting)}

      {idleCollecting ? (
        <Stack spacing={1.5} alignItems="center" sx={{ py: 1 }}>
          <CircularProgress size={36} />
          <Typography variant="h5" component="p">
            {t("gamepad.idleCountdown", { seconds: idleSecondsLeft })}
          </Typography>
          <Typography align="center" color="text.secondary">
            {t("gamepad.idleWaitHint")}
          </Typography>
        </Stack>
      ) : (
        <Stack spacing={1.5}>
          <Typography color="text.secondary">
            {t("gamepad.idleIntro")}
          </Typography>
          <Button variant="contained" onClick={startIdleLearning}>
            {t("gamepad.learnIdle")}
          </Button>
        </Stack>
      )}

      {idleError && (
        <Typography variant="body2" color="error">
          {idleError}
        </Typography>
      )}
    </Stack>
  );

  const renderSettingsStep = () => (
    <>
      <Box>
        <Typography variant="subtitle2" gutterBottom>
          {t("gamepad.connected")}
        </Typography>
        <Typography variant="body2">
          {activePad?.id ?? draft.gamepadId}
        </Typography>
        {draft.idleAxisMin != null && draft.idleAxisMax != null && (
          <Typography variant="caption" color="text.secondary" display="block">
            {t("gamepad.idleRange", {
              min: draft.idleAxisMin.toFixed(3),
              max: draft.idleAxisMax.toFixed(3),
            })}
          </Typography>
        )}
      </Box>

      {draft.axisEnabled !== false && renderAxisVisualizer(false)}

      <Box>
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{ mb: 1 }}
        >
          <Typography variant="subtitle2">{t("gamepad.speedAxis")}</Typography>
          <FormControlLabel
            control={
              <Switch
                size="small"
                checked={draft.axisEnabled !== false}
                onChange={(_, checked) =>
                  updateDraft({ axisEnabled: checked })
                }
              />
            }
            label={t("gamepad.axisEnabled")}
            sx={{ mr: 0 }}
          />
        </Stack>
        {draft.axisEnabled !== false ? (
          <>
            <Typography variant="body2" color="text.secondary">
              {t("gamepad.speedAxisValue", {
                axis: draft.speedAxis,
                value: previewAxis.toFixed(2),
                speed: previewSpeed,
              })}
            </Typography>
            <Stack direction="row" spacing={1} sx={{ mt: 1 }}>
              <Button
                size="small"
                variant={learn?.kind === "axis" ? "contained" : "outlined"}
                onClick={() => {
                  learnBaselineRef.current = null;
                  setLearn({ kind: "axis" });
                }}
              >
                {t("gamepad.learnAxis")}
              </Button>
              <FormControlLabel
                control={
                  <Switch
                    size="small"
                    checked={draft.invertAxis}
                    onChange={(_, checked) =>
                      updateDraft({ invertAxis: checked })
                    }
                  />
                }
                label={t("gamepad.invertAxis")}
              />
            </Stack>
          </>
        ) : (
          <Typography variant="body2" color="text.secondary">
            {t("gamepad.axisDisabledHint")}
          </Typography>
        )}
      </Box>

      {draft.axisEnabled !== false && (
      <Box>
        <Typography variant="subtitle2" gutterBottom>
          {t("gamepad.sensitivity")}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
          {t("gamepad.sensitivityHelp", {
            scale: formatSpeedSensitivityDivisor(
              GAMEPAD_SPEED_SENSITIVITY_DIVISORS[draft.speedSensitivity ?? 0],
            ),
          })}
        </Typography>
        <Slider
          value={draft.speedSensitivity ?? 0}
          onChange={(_, value) =>
            updateDraft({
              speedSensitivity: value as GamepadSpeedSensitivity,
            })
          }
          step={1}
          min={0}
          max={GAMEPAD_SPEED_SENSITIVITY_MAX}
          marks={GAMEPAD_SPEED_SENSITIVITY_DIVISORS.map((divisor, index) => ({
            value: index,
            label: formatSpeedSensitivityDivisor(divisor),
          }))}
          valueLabelDisplay="off"
          sx={{ mx: 1 }}
        />
      </Box>
      )}

      <Box>
        <Typography variant="subtitle2" gutterBottom>
          {t("gamepad.speedButtons")}
        </Typography>
        <TextField
          label={t("gamepad.speedButtonSteps")}
          type="number"
          size="small"
          value={draft.speedButtonSteps ?? DEFAULT_SPEED_BUTTON_STEPS}
          onChange={(e) =>
            updateDraft({
              speedButtonSteps: parseSpeedButtonSteps(Number(e.target.value)),
            })
          }
          helperText={t("gamepad.speedButtonStepsHelp", {
            step: speedButtonStepSize(
              maxSpeed,
              draft.speedButtonSteps ?? DEFAULT_SPEED_BUTTON_STEPS,
            ),
            max: maxSpeed,
          })}
          inputProps={{
            min: MIN_SPEED_BUTTON_STEPS,
            max: MAX_SPEED_BUTTON_STEPS,
            step: 1,
          }}
          sx={{ mb: 1.5, maxWidth: 200 }}
        />
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Button
            size="small"
            variant={learn?.kind === "accelerate" ? "contained" : "outlined"}
            onClick={() => setLearn({ kind: "accelerate" })}
          >
            {draft.accelerateButton != null
              ? t("gamepad.accelerateAssigned", {
                  button: buttonLabel(draft.accelerateButton),
                })
              : t("gamepad.assignAccelerate")}
          </Button>
          {draft.accelerateButton != null && (
            <Button
              size="small"
              onClick={() => updateDraft({ accelerateButton: undefined })}
            >
              {t("gamepad.clear")}
            </Button>
          )}
          <Button
            size="small"
            variant={learn?.kind === "decelerate" ? "contained" : "outlined"}
            onClick={() => setLearn({ kind: "decelerate" })}
          >
            {draft.decelerateButton != null
              ? t("gamepad.decelerateAssigned", {
                  button: buttonLabel(draft.decelerateButton),
                })
              : t("gamepad.assignDecelerate")}
          </Button>
          {draft.decelerateButton != null && (
            <Button
              size="small"
              onClick={() => updateDraft({ decelerateButton: undefined })}
            >
              {t("gamepad.clear")}
            </Button>
          )}
        </Stack>
      </Box>

      <Box>
        <Typography variant="subtitle2" gutterBottom>
          {t("gamepad.extraButtons")}
        </Typography>
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Button
            size="small"
            variant={learn?.kind === "reverse" ? "contained" : "outlined"}
            onClick={() => setLearn({ kind: "reverse" })}
          >
            {draft.reverseButton != null
              ? t("gamepad.reverseAssigned", {
                  button: buttonLabel(draft.reverseButton),
                })
              : t("gamepad.assignReverse")}
          </Button>
          {draft.reverseButton != null && (
            <Button
              size="small"
              onClick={() => updateDraft({ reverseButton: undefined })}
            >
              {t("gamepad.clear")}
            </Button>
          )}
          <Button
            size="small"
            variant={learn?.kind === "stop" ? "contained" : "outlined"}
            onClick={() => setLearn({ kind: "stop" })}
          >
            {draft.stopButton != null
              ? t("gamepad.stopAssigned", {
                  button: buttonLabel(draft.stopButton),
                })
              : t("gamepad.assignStop")}
          </Button>
          {draft.stopButton != null && (
            <Button
              size="small"
              onClick={() => updateDraft({ stopButton: undefined })}
            >
              {t("gamepad.clear")}
            </Button>
          )}
        </Stack>
      </Box>

      {configuredFunctions.length > 0 && (
        <Box>
          <Typography variant="subtitle2" gutterBottom>
            {t("gamepad.functions")}
          </Typography>
          <Stack spacing={1}>
            {configuredFunctions.map((fn) => {
              const assigned = draft.fnButtons[fn.num];
              const isLearning = learn?.kind === "fn" && learn.num === fn.num;
              return (
                <Stack
                  key={fn.num}
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  flexWrap="wrap"
                  useFlexGap
                >
                  <Typography
                    variant="body2"
                    sx={{ minWidth: 120, flex: "1 1 120px" }}
                  >
                    {t("fnLabel", { n: fn.num })}
                    {fn.label ? ` — ${fn.label}` : ""}
                  </Typography>
                  <Button
                    size="small"
                    variant={isLearning ? "contained" : "outlined"}
                    onClick={() => setLearn({ kind: "fn", num: fn.num })}
                  >
                    {assigned != null
                      ? t("gamepad.buttonAssigned", {
                          button: buttonLabel(assigned),
                        })
                      : t("gamepad.assign")}
                  </Button>
                  {assigned != null && (
                    <Button
                      size="small"
                      onClick={() => {
                        const next = { ...draft.fnButtons };
                        delete next[fn.num];
                        updateDraft({ fnButtons: next });
                      }}
                    >
                      {t("gamepad.clear")}
                    </Button>
                  )}
                </Stack>
              );
            })}
          </Stack>
        </Box>
      )}

      <FormControlLabel
        control={
          <Switch
            checked={draft.enabled}
            onChange={(_, checked) => updateDraft({ enabled: checked })}
          />
        }
        label={t("gamepad.enable")}
      />
    </>
  );

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          pr: 1,
        }}
      >
        <Stack direction="row" spacing={1} alignItems="center">
          <SportsEsportsIcon fontSize="small" />
          <Typography variant="h6" component="span">
            {t("gamepad.title")}
          </Typography>
        </Stack>
        <IconButton edge="end" onClick={onClose} aria-label={t("setup.close")}>
          <CloseIcon />
        </IconButton>
      </DialogTitle>

      <DialogContent dividers>
        <Stack spacing={2.5}>
          {step === "warning" && renderWarningStep()}
          {step === "detect" && renderDetectStep()}
          {step === "idle" && renderIdleStep()}
          {step === "settings" && renderSettingsStep()}

          {learnHint && (
            <Typography variant="body2" color="primary">
              {learnHint}
            </Typography>
          )}
        </Stack>
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>{t("gamepad.cancel")}</Button>
        {step === "warning" && (
          <Button variant="contained" onClick={acknowledgeWarning}>
            {t("gamepad.safetyAcknowledge")}
          </Button>
        )}
        {step === "settings" && (
          <Button variant="contained" onClick={handleConfirm}>
            {t("gamepad.confirm")}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
