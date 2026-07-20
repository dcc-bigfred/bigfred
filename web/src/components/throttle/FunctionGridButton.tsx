import { memo, useCallback } from "react";
import { Box, Typography } from "@mui/material";

import { FunctionIconGlyph } from "../functions/FunctionIconGlyph";
import {
  cockpit,
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
  const top = active ? cockpit.btnActiveTop : cockpit.btnTop;
  const bottom = active ? cockpit.btnActiveBottom : cockpit.btnBottom;

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
        border: `1px solid ${active ? cockpit.borderBright : cockpit.border}`,
        borderRadius: 1.5,
        cursor: disabled ? "not-allowed" : "pointer",
        background: `linear-gradient(145deg, ${top} 0%, ${bottom} 100%)`,
        boxShadow: active
          ? "inset 0 2px 8px rgba(255,255,255,0.2), 0 2px 6px rgba(0,0,0,0.4)"
          : "inset 0 1px 0 rgba(255,255,255,0.15), 0 2px 4px rgba(0,0,0,0.35)",
        opacity: disabled ? 0.45 : 1,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "flex-end",
        overflow: "hidden",
        // Prefer solid overlay over filter:brightness — cheaper on old GPUs.
        "&:hover:not(:disabled)": {
          background: `linear-gradient(145deg, ${top} 0%, ${bottom} 100%)`,
          boxShadow: active
            ? "inset 0 2px 8px rgba(255,255,255,0.28), 0 2px 6px rgba(0,0,0,0.4)"
            : "inset 0 1px 0 rgba(255,255,255,0.22), 0 2px 4px rgba(0,0,0,0.35)",
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
          color: cockpit.textMuted,
          fontWeight: 700,
          fontSize: FUNCTION_BUTTON_NUM_FONT_SIZE_PX,
          lineHeight: 1,
          letterSpacing: "0.02em",
          // Hard 1px offset (no blur) — blurred text-shadow causes white
          // speckles on older Android WebViews over dark gradients.
          textShadow: "0 1px 0 rgba(0,0,0,0.9)",
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
          // Soft glow via opacity bump instead of filter:drop-shadow (GPU-heavy / speckles).
          opacity: active ? 1 : undefined,
        }}
      >
        <FunctionIconGlyph
          slug={icon}
          size={COCKPIT_FUNCTION_ICON_PX}
          variant="cockpit"
          active={active}
        />
      </Box>
      <Typography
        variant="caption"
        component="span"
        sx={{
          position: "relative",
          zIndex: 1,
          color: cockpit.text,
          fontWeight: 600,
          fontSize: FUNCTION_BUTTON_LABEL_FONT_SIZE_PX,
          lineHeight: 1.15,
          textAlign: "center",
          maxWidth: "100%",
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          px: 0.25,
          textShadow: "0 1px 0 rgba(0,0,0,0.85)",
        }}
      >
        {label}
      </Typography>
    </Box>
  );
}

export default memo(FunctionGridButton);
