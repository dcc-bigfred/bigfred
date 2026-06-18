import { useCallback, useMemo, useState, type ReactNode } from "react";
import {
  Box,
  CircularProgress,
  FormControl,
  IconButton,
  ListSubheader,
  MenuItem,
  Select,
  Typography,
  type SelectChangeEvent,
} from "@mui/material";
import ChevronLeftIcon from "@mui/icons-material/ChevronLeft";
import ChevronRightIcon from "@mui/icons-material/ChevronRight";
import SettingsIcon from "@mui/icons-material/Settings";
import SyncIcon from "@mui/icons-material/Sync";
import TuneIcon from "@mui/icons-material/Tune";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";

import type { ThrottleTarget } from "../../hooks/useThrottleTargetSelection";
import {
  cockpit,
  FUNCTION_BUTTON_GRID_GAP_PX,
  FUNCTION_BUTTON_SIZE_PX,
  THROTTLE_PANEL_WIDTH_PX,
} from "./throttleCockpitTheme";
import RadioStopButton from "./RadioStopButton";
import FunctionGridButton from "./FunctionGridButton";
import VerticalThrottle from "./VerticalThrottle";

export interface ThrottleCockpitFunction {
  num: number;
  label: string;
  icon: string;
}

export interface ThrottleCockpitVehicle {
  id: number;
  name: string;
  dccAddress: number;
}

export interface ThrottleCockpitTrain {
  id: number;
  name: string;
}

export interface ThrottleCockpitProps {
  layoutId: number;
  onOpenSetup: () => void;
  vehicles: ThrottleCockpitVehicle[];
  /** When omitted, vehicle-only picker mode (legacy). */
  trains?: ThrottleCockpitTrain[];
  selectedTarget?: ThrottleTarget | null;
  onSelectTarget?: (target: ThrottleTarget) => void;
  /** Legacy vehicle-only selection — used when selectedTarget is not provided. */
  selectedAddress?: number | null;
  onSelectAddress?: (address: number) => void;
  speed: number;
  maxSpeed: number;
  forward: boolean;
  functions: boolean[];
  configuredFunctions: ThrottleCockpitFunction[];
  /** Replaces the flat function grid (train accordion mode). */
  functionPanel?: ReactNode;
  disabled?: boolean;
  /** When true, settings icon shows a reconnect spinner instead. */
  connectionLost?: boolean;
  /** When true, settings icon shows a retry spinner (command resend in flight). */
  commandRetrying?: boolean;
  onSpeedChange: (speed: number) => void;
  onDirectionChange: (forward: boolean) => void;
  onFunctionToggle: (fn: number) => void;
  onStop: () => void;
  headerExtra?: ReactNode;
}

function targetKey(target: ThrottleTarget | null): string {
  if (target == null) return "";
  return target.kind === "vehicle"
    ? `v:${target.dccAddress}`
    : `t:${target.trainId}`;
}

export default function ThrottleCockpit({
  layoutId,
  onOpenSetup,
  vehicles,
  trains = [],
  selectedTarget,
  onSelectTarget,
  selectedAddress = null,
  onSelectAddress,
  speed,
  maxSpeed,
  forward,
  functions,
  configuredFunctions,
  functionPanel,
  disabled = false,
  connectionLost = false,
  commandRetrying = false,
  onSpeedChange,
  onDirectionChange,
  onFunctionToggle,
  onStop,
  headerExtra,
}: ThrottleCockpitProps) {
  const { t } = useTranslation("throttle");
  const navigate = useNavigate();
  const [radioStopRetrying, setRadioStopRetrying] = useState(false);
  const onRadioStopRetryingChange = useCallback(
    (retrying: boolean) => setRadioStopRetrying(retrying),
    [],
  );
  const settingsRetrying = commandRetrying || radioStopRetrying;

  const effectiveTarget: ThrottleTarget | null = useMemo(() => {
    if (selectedTarget != null) return selectedTarget;
    if (selectedAddress != null) {
      return { kind: "vehicle", dccAddress: selectedAddress };
    }
    return null;
  }, [selectedTarget, selectedAddress]);

  const selectedVehicle = useMemo(() => {
    if (effectiveTarget?.kind !== "vehicle") return undefined;
    return vehicles.find((v) => v.dccAddress === effectiveTarget.dccAddress);
  }, [vehicles, effectiveTarget]);

  const selectedTrain = useMemo(() => {
    if (effectiveTarget?.kind !== "train") return undefined;
    return trains.find((tr) => tr.id === effectiveTarget.trainId);
  }, [trains, effectiveTarget]);

  const pickerLabel =
    selectedTrain?.name ?? selectedVehicle?.name ?? t("vehicle");

  const hasSelection = effectiveTarget != null;
  const dualPicker = onSelectTarget != null && trains.length >= 0;

  const speedPercent =
    maxSpeed > 0 ? Math.round((speed / maxSpeed) * 100) : 0;

  const handlePickerChange = (ev: SelectChangeEvent<string>) => {
    const raw = ev.target.value;
    if (!onSelectTarget) {
      const addr = Number(raw);
      if (Number.isFinite(addr) && addr > 0) {
        onSelectAddress?.(addr);
      }
      return;
    }
    if (raw.startsWith("v:")) {
      const addr = Number(raw.slice(2));
      if (Number.isFinite(addr) && addr > 0) {
        onSelectTarget({ kind: "vehicle", dccAddress: addr });
      }
      return;
    }
    if (raw.startsWith("t:")) {
      const trainId = Number(raw.slice(2));
      if (Number.isFinite(trainId) && trainId > 0) {
        onSelectTarget({ kind: "train", trainId });
      }
    }
  };

  return (
    <Box
      sx={{
        bgcolor: cockpit.bg,
        overflow: "hidden",
        display: "flex",
        flexDirection: "column",
        flex: 1,
        width: "100%",
        height: "100%",
        minHeight: 0,
      }}
    >
      <Box
        component="header"
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1,
          px: 1.5,
          py: 1,
          flexShrink: 0,
          bgcolor: cockpit.header,
          borderBottom: `1px solid ${cockpit.border}`,
          minHeight: 48,
        }}
      >
        <RadioStopButton
          layoutId={layoutId}
          onRetryingChange={onRadioStopRetryingChange}
        />
        {headerExtra}

        <FormControl
          size="small"
          disabled={
            disabled ||
            (vehicles.length === 0 && trains.length === 0)
          }
          sx={{
            flex: 1,
            minWidth: 0,
            "& .MuiOutlinedInput-notchedOutline": { border: "none" },
            "& .MuiSelect-select": {
              py: 0.75,
              textAlign: "center",
              color: cockpit.text,
              fontWeight: 600,
              fontSize: "1.05rem",
            },
            "& .MuiSvgIcon-root": { color: cockpit.textMuted },
          }}
        >
          <Select
            value={targetKey(effectiveTarget)}
            displayEmpty
            onChange={handlePickerChange}
            renderValue={() => pickerLabel}
            aria-label={t("vehicle")}
          >
            {dualPicker && vehicles.length > 0 && (
              <ListSubheader>{t("vehicle")}</ListSubheader>
            )}
            {vehicles.map((v) => (
              <MenuItem
                key={v.id}
                value={dualPicker ? `v:${v.dccAddress}` : String(v.dccAddress)}
              >
                {v.name} ({v.dccAddress})
              </MenuItem>
            ))}
            {dualPicker && trains.length > 0 && (
              <ListSubheader>{t("train.picker")}</ListSubheader>
            )}
            {dualPicker &&
              trains.map((tr) => (
                <MenuItem key={tr.id} value={`t:${tr.id}`}>
                  {tr.name}
                </MenuItem>
              ))}
          </Select>
        </FormControl>

        <IconButton
          size="small"
          onClick={() => {
            if (selectedVehicle) {
              navigate(`/my/vehicles/${selectedVehicle.id}/functions`);
            }
          }}
          disabled={disabled || selectedVehicle == null}
          aria-label={t("editFunctions")}
          sx={{ color: cockpit.text }}
        >
          <TuneIcon fontSize="small" />
        </IconButton>

        <IconButton
          size="small"
          onClick={onOpenSetup}
          disabled={connectionLost || settingsRetrying}
          aria-label={
            connectionLost
              ? t("reconnecting")
              : settingsRetrying
                ? t("commandRetrying")
                : t("setup.open")
          }
          sx={{ color: cockpit.text }}
        >
          {connectionLost ? (
            <CircularProgress size={18} sx={{ color: cockpit.text }} />
          ) : settingsRetrying ? (
            <SyncIcon
              fontSize="small"
              sx={{
                color: cockpit.text,
                animation: "throttleCommandRetrySpin 1s linear infinite",
                "@keyframes throttleCommandRetrySpin": {
                  from: { transform: "rotate(0deg)" },
                  to: { transform: "rotate(360deg)" },
                },
              }}
            />
          ) : (
            <SettingsIcon fontSize="small" />
          )}
        </IconButton>
      </Box>

      <Box
        sx={{
          display: "flex",
          flex: 1,
          flexDirection: "row",
          minHeight: 0,
          minWidth: 0,
          overflow: "hidden",
        }}
      >
        <Box
          sx={{
            flex: "1 1 0",
            minWidth: 0,
            minHeight: 0,
            overflowY: "auto",
            overflowX: "hidden",
            overscrollBehavior: "contain",
            WebkitOverflowScrolling: "touch",
            p: 1.25,
            bgcolor: cockpit.bgPanel,
          }}
        >
          {functionPanel ?? (
            <Box
              sx={{
                display: "grid",
                width: "100%",
                gridTemplateColumns: `repeat(auto-fill, ${FUNCTION_BUTTON_SIZE_PX}px)`,
                gap: `${FUNCTION_BUTTON_GRID_GAP_PX}px`,
                justifyContent: "start",
              }}
            >
              {configuredFunctions.map((fn) => (
                <FunctionGridButton
                  key={fn.num}
                  fnCode={t("fnLabel", { n: fn.num })}
                  label={fn.label}
                  icon={fn.icon}
                  active={Boolean(functions[fn.num])}
                  disabled={disabled || !hasSelection}
                  onClick={() => onFunctionToggle(fn.num)}
                />
              ))}
            </Box>
          )}
        </Box>

        <Box
          sx={{
            flex: `0 0 ${THROTTLE_PANEL_WIDTH_PX}px`,
            width: THROTTLE_PANEL_WIDTH_PX,
            minWidth: THROTTLE_PANEL_WIDTH_PX,
            maxWidth: THROTTLE_PANEL_WIDTH_PX,
            display: "flex",
            flexDirection: "column",
            minHeight: 0,
            maxHeight: "100%",
            flexShrink: 0,
            overflow: "hidden",
            alignSelf: "stretch",
            borderLeft: `1px solid ${cockpit.border}`,
            bgcolor: cockpit.bg,
          }}
        >
          <Box
            sx={{
              flex: 1,
              minHeight: 0,
              width: "100%",
              position: "relative",
              display: "flex",
              flexDirection: "column",
              pt: 1.5,
              px: 0.5,
            }}
          >
            <VerticalThrottle
              value={speed}
              max={maxSpeed}
              disabled={disabled || !hasSelection}
              onChange={onSpeedChange}
            />
            <Box
              aria-hidden
              sx={{
                position: "absolute",
                top: 12,
                right: 0,
                bottom: 4,
                width: 14,
                borderRadius: 1,
                background: cockpit.speedGradient,
                boxShadow: `inset 0 0 6px rgba(0,0,0,0.45)`,
                opacity: 0.9,
                pointerEvents: "none",
              }}
            />
          </Box>

          <Box sx={{ flexShrink: 0, px: 1.5, pb: 1.5, mt: "45px" }}>
            <Box
              component="button"
              type="button"
              disabled={disabled || !hasSelection}
              onClick={onStop}
              sx={{
                width: "100%",
                py: 1.25,
                border: `1px solid ${cockpit.border}`,
                borderRadius: 1,
                bgcolor: cockpit.btnBottom,
                background: `linear-gradient(180deg, ${cockpit.btnTop} 0%, ${cockpit.btnBottom} 100%)`,
                color: cockpit.text,
                fontWeight: 600,
                fontSize: "1rem",
                cursor: "pointer",
                boxShadow:
                  "inset 0 1px 0 rgba(255,255,255,0.12), 0 2px 4px rgba(0,0,0,0.35)",
                "&:hover:not(:disabled)": {
                  filter: "brightness(1.08)",
                },
                "&:disabled": {
                  opacity: 0.45,
                  cursor: "not-allowed",
                },
              }}
            >
              {t("stop")}
            </Box>
          </Box>
        </Box>
      </Box>

      <Box
        component="footer"
        sx={{
          flexShrink: 0,
          display: "flex",
          alignItems: "center",
          width: "100%",
          minHeight: 56,
          px: 2,
          py: 1,
          gap: 2,
          borderTop: `1px solid ${cockpit.border}`,
          bgcolor: cockpit.header,
        }}
      >
        <Box sx={{ flex: 1, display: "flex", justifyContent: "flex-start" }}>
          <IconButton
            disabled={disabled || !hasSelection}
            onClick={() => onDirectionChange(false)}
            aria-label={t("direction.reverse")}
            sx={{
              color: !forward ? cockpit.accent : cockpit.textMuted,
              border: `1px solid ${!forward ? cockpit.accent : cockpit.border}`,
              borderRadius: 1,
              px: 2,
            }}
          >
            <ChevronLeftIcon />
            <ChevronLeftIcon sx={{ ml: -1.25 }} />
          </IconButton>
        </Box>

        <Typography
          variant="h6"
          component="span"
          sx={{
            color: cockpit.text,
            fontWeight: 700,
            fontVariantNumeric: "tabular-nums",
            flexShrink: 0,
            textAlign: "center",
          }}
        >
          {speedPercent}%
        </Typography>

        <Box sx={{ flex: 1, display: "flex", justifyContent: "flex-end" }}>
          <IconButton
            disabled={disabled || !hasSelection}
            onClick={() => onDirectionChange(true)}
            aria-label={t("direction.forward")}
            sx={{
              color: forward ? cockpit.accent : cockpit.textMuted,
              border: `1px solid ${forward ? cockpit.accent : cockpit.border}`,
              borderRadius: 1,
              px: 2,
            }}
          >
            <ChevronRightIcon />
            <ChevronRightIcon sx={{ ml: -1.25 }} />
          </IconButton>
        </Box>
      </Box>
    </Box>
  );
}
