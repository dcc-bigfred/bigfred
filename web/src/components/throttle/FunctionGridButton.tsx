import { memo, useCallback } from "react";
import { Box, Typography } from "@mui/material";

import { FunctionIconGlyph } from "../functions/FunctionIconGlyph";
import {
  cockpit,
  cockpitFunctionSurface,
  COCKPIT_FUNCTION_ICON_PX,
  FUNCTION_BUTTON_ICON_TOP_PX,
  FUNCTION_BUTTON_LABEL_FONT_SIZE_PX,
  FUNCTION_BUTTON_NUM_FONT_SIZE_PX,
  FUNCTION_BUTTON_NUM_INSET_PX,
  FUNCTION_BUTTON_SIZE_PX,
} from "./throttleCockpitTheme";

interface FunctionGridButtonProps {
  /** DCC function index label, e.g. `F0`, `F6`. */
  fnCode: string;
  /** DCC function number passed to onToggle. */
  fnNum: number;
  label: string;
  icon: string;
  active: boolean;
  disabled?: boolean;
  onToggle: (fn: number) => void;
}

// FunctionGridButton is one DCC function cell in the throttle cockpit.
function FunctionGridButton({
  fnCode,
  fnNum,
  label,
  icon,
  active,
  disabled,
  onToggle,
}: FunctionGridButtonProps) {
  const surface = cockpitFunctionSurface(active, Boolean(disabled));

  const handleClick = useCallback(() => {
    onToggle(fnNum);
  }, [onToggle, fnNum]);

  return (
    <Box
      component="button"
      type="button"
      disabled={disabled}
      onClick={handleClick}
      aria-pressed={active}
      aria-label={`${fnCode} ${label}`}
      title={`${fnCode} — ${label}`}
      sx={{
        position: "relative",
        width: FUNCTION_BUTTON_SIZE_PX,
        height: FUNCTION_BUTTON_SIZE_PX,
        minWidth: FUNCTION_BUTTON_SIZE_PX,
        minHeight: FUNCTION_BUTTON_SIZE_PX,
        flexShrink: 0,
        p: 0.5,
        boxSizing: "border-box",
        border: surface.border,
        borderRadius: 1.5,
        cursor: disabled ? "not-allowed" : "pointer",
        bgcolor: surface.bgcolor,
        boxShadow: "none",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "flex-end",
        overflow: "hidden",
        // Solid color bump — no filter:brightness / white inset (old WebView speckles).
        "&:hover:not(:disabled)": {
          bgcolor: active ? cockpit.fnFillActiveHover : cockpit.fnFillHover,
          border: `1px solid ${active ? cockpit.fnBorderActive : cockpit.fnBorder}`,
        },
      }}
    >
      <Typography
        component="span"
        aria-hidden
        sx={{
          position: "absolute",
          top: FUNCTION_BUTTON_NUM_INSET_PX,
          left: FUNCTION_BUTTON_NUM_INSET_PX,
          zIndex: 2,
          color: disabled ? cockpit.textDisabled : cockpit.textMuted,
          fontWeight: 700,
          fontSize: FUNCTION_BUTTON_NUM_FONT_SIZE_PX,
          lineHeight: 1,
          letterSpacing: "0.02em",
          pointerEvents: "none",
          userSelect: "none",
        }}
      >
        {fnCode}
      </Typography>
      <Box
        sx={{
          position: "absolute",
          top: FUNCTION_BUTTON_ICON_TOP_PX,
          // left+right + flex centering avoids translateX(-50%) subpixel text/icon artifacts.
          left: 0,
          right: 0,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <FunctionIconGlyph
          slug={icon}
          size={COCKPIT_FUNCTION_ICON_PX}
          variant="cockpit"
          active={active && !disabled}
        />
      </Box>
      <Typography
        variant="caption"
        component="span"
        sx={{
          position: "relative",
          zIndex: 1,
          color: disabled ? cockpit.textDisabled : cockpit.text,
          fontWeight: 600,
          fontSize: FUNCTION_BUTTON_LABEL_FONT_SIZE_PX,
          lineHeight: 1.15,
          textAlign: "center",
          maxWidth: "100%",
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          px: 0.25,
        }}
      >
        {label}
      </Typography>
    </Box>
  );
}

export default memo(FunctionGridButton);
