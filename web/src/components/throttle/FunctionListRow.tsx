import { memo, useCallback } from "react";
import { Box, Typography } from "@mui/material";

import { FunctionIconGlyph } from "../functions/FunctionIconGlyph";
import {
  cockpit,
  cockpitFunctionSurface,
  FUNCTION_LIST_ICON_PX,
} from "./throttleCockpitTheme";

interface FunctionListRowProps {
  /** DCC function index label, e.g. `F0`. */
  fnCode: string;
  /** DCC function number passed to onToggle. */
  fnNum: number;
  label: string;
  icon: string;
  active: boolean;
  disabled?: boolean;
  onToggle: (fn: number) => void;
}

// Compact function row for the Throttle list view (icon + "Fn: name").
function FunctionListRow({
  fnCode,
  fnNum,
  label,
  icon,
  active,
  disabled,
  onToggle,
}: FunctionListRowProps) {
  const text = `${fnCode}: ${label}`;
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
      aria-label={text}
      title={text}
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1,
        width: "100%",
        minHeight: 40,
        px: 1,
        py: 0.5,
        boxSizing: "border-box",
        border: surface.border,
        borderRadius: 0.5,
        cursor: disabled ? "not-allowed" : "pointer",
        bgcolor: surface.bgcolor,
        boxShadow: "none",
        textAlign: "left",
      }}
    >
      <FunctionIconGlyph
        slug={icon}
        size={FUNCTION_LIST_ICON_PX}
        variant="cockpit"
        active={active && !disabled}
      />
      <Typography
        component="span"
        sx={{
          flex: 1,
          minWidth: 0,
          color: disabled ? cockpit.textDisabled : cockpit.text,
          fontWeight: 600,
          fontSize: 13,
          lineHeight: 1.2,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        {text}
      </Typography>
    </Box>
  );
}

export default memo(FunctionListRow);
