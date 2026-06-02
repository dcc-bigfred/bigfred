import { useMemo } from "react";
import {
  Box,
  FormControl,
  IconButton,
  MenuItem,
  Select,
  Typography,
  type SelectChangeEvent,
} from "@mui/material";
import ChevronLeftIcon from "@mui/icons-material/ChevronLeft";
import ChevronRightIcon from "@mui/icons-material/ChevronRight";
import SettingsIcon from "@mui/icons-material/Settings";
import { useTranslation } from "react-i18next";

import {
  cockpit,
  FUNCTION_BUTTON_SIZE_PX,
  THROTTLE_PANEL_WIDTH_PX,
} from "./throttleCockpitTheme";
import FunctionGridButton from "./FunctionGridButton";
import VerticalThrottle from "./VerticalThrottle";

const FUNCTION_COUNT = 33; // F0 … F32

export interface ThrottleCockpitVehicle {
  id: number;
  name: string;
  dccAddress: number;
}

export interface ThrottleCockpitProps {
  onOpenSetup: () => void;
  vehicles: ThrottleCockpitVehicle[];
  selectedAddress: number | null;
  onSelectAddress: (address: number) => void;
  speed: number;
  maxSpeed: number;
  forward: boolean;
  functions: boolean[];
  disabled?: boolean;
  onSpeedChange: (speed: number) => void;
  onDirectionChange: (forward: boolean) => void;
  onFunctionToggle: (fn: number) => void;
  onStop: () => void;
}

// ThrottleCockpit is the locomotive-control surface: function grid on
// the left, vertical throttle + stop on the right, direction bar full
// width at the bottom.
export default function ThrottleCockpit({
  onOpenSetup,
  vehicles,
  selectedAddress,
  onSelectAddress,
  speed,
  maxSpeed,
  forward,
  functions,
  disabled = false,
  onSpeedChange,
  onDirectionChange,
  onFunctionToggle,
  onStop,
}: ThrottleCockpitProps) {
  const { t } = useTranslation("throttle");

  const selectedVehicle = useMemo(
    () => vehicles.find((v) => v.dccAddress === selectedAddress),
    [vehicles, selectedAddress],
  );

  const speedPercent =
    maxSpeed > 0 ? Math.round((speed / maxSpeed) * 100) : 0;

  const handleVehicleChange = (ev: SelectChangeEvent<string>) => {
    const addr = Number(ev.target.value);
    if (Number.isFinite(addr) && addr > 0) {
      onSelectAddress(addr);
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
        <FormControl
          size="small"
          disabled={disabled || vehicles.length === 0}
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
            value={
              selectedAddress != null ? String(selectedAddress) : ""
            }
            displayEmpty
            onChange={handleVehicleChange}
            renderValue={() =>
              selectedVehicle?.name ??
              t("vehicle")
            }
            aria-label={t("vehicle")}
          >
            {vehicles.map((v) => (
              <MenuItem key={v.id} value={String(v.dccAddress)}>
                {v.name} ({v.dccAddress})
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <IconButton
          size="small"
          onClick={onOpenSetup}
          aria-label={t("setup.open")}
          sx={{ color: cockpit.text }}
        >
          <SettingsIcon fontSize="small" />
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
          <Box
            sx={{
              display: "grid",
              width: "100%",
              gridTemplateColumns: `repeat(auto-fill, ${FUNCTION_BUTTON_SIZE_PX}px)`,
              gap: 1,
              justifyContent: "start",
            }}
          >
            {Array.from({ length: FUNCTION_COUNT }, (_, n) => (
              <FunctionGridButton
                key={n}
                label={t("fnLabel", { n })}
                active={Boolean(functions[n])}
                disabled={disabled || selectedAddress == null}
                onClick={() => onFunctionToggle(n)}
              />
            ))}
          </Box>
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
              disabled={disabled || selectedAddress == null}
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

          <Box sx={{ flexShrink: 0, px: 1.5, pb: 1.5, mt: "20px" }}>
            <Box
              component="button"
              type="button"
              disabled={disabled || selectedAddress == null}
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
            disabled={disabled || selectedAddress == null}
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
            disabled={disabled || selectedAddress == null}
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
