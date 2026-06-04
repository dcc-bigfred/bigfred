import { Box, Typography } from "@mui/material";

import {
  COCKPIT_FUNCTION_ICON_PX,
  FunctionIconGlyph,
} from "../functions/FunctionIconGlyph";
import { cockpit, FUNCTION_BUTTON_SIZE_PX } from "./throttleCockpitTheme";

interface FunctionGridButtonProps {
  label: string;
  icon: string;
  active: boolean;
  disabled?: boolean;
  onClick: () => void;
}

// FunctionGridButton is one DCC function cell in the throttle cockpit.
export default function FunctionGridButton({
  label,
  icon,
  active,
  disabled,
  onClick,
}: FunctionGridButtonProps) {
  const top = active ? cockpit.btnActiveTop : cockpit.btnTop;
  const bottom = active ? cockpit.btnActiveBottom : cockpit.btnBottom;

  return (
    <Box
      component="button"
      type="button"
      disabled={disabled}
      onClick={onClick}
      aria-pressed={active}
      title={label}
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
        "&:hover:not(:disabled)": {
          filter: "brightness(1.06)",
        },
      }}
    >
      <Box
        sx={{
          position: "absolute",
          top: "14%",
          left: "50%",
          transform: "translateX(-50%)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          filter: active
            ? "drop-shadow(0 0 6px rgba(232, 240, 252, 0.45))"
            : "none",
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
          fontSize: "0.65rem",
          lineHeight: 1.15,
          textAlign: "center",
          maxWidth: "100%",
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          px: 0.25,
          textShadow: "0 1px 2px rgba(0,0,0,0.6)",
        }}
      >
        {label}
      </Typography>
    </Box>
  );
}
