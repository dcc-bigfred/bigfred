import { useMemo } from "react";
import { IconButton, Tooltip } from "@mui/material";
import ScreenRotationIcon from "@mui/icons-material/ScreenRotation";
import { useTranslation } from "react-i18next";

import {
  canRotateScreen,
  rotateScreen,
} from "../../native/bigfredNativeApp";
import { cockpit } from "./throttleCockpitTheme";

/**
 * Manual portrait/landscape toggle for the Android shell.
 * Renders nothing in a normal browser (no Screen Orientation API fallback).
 */
export default function ThrottleRotateScreenButton() {
  const { t } = useTranslation("throttle");
  const available = useMemo(() => canRotateScreen(), []);

  if (!available) {
    return null;
  }

  const label = t("rotateScreen.tooltip");

  return (
    <Tooltip title={label}>
      <IconButton
        onClick={() => rotateScreen()}
        aria-label={label}
        size="small"
        sx={{
          color: cockpit.text,
          "&:hover": { bgcolor: "rgba(255,255,255,0.08)" },
        }}
      >
        <ScreenRotationIcon fontSize="small" />
      </IconButton>
    </Tooltip>
  );
}
