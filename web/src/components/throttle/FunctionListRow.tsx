import { memo, useCallback } from "react";
import { Box, Typography } from "@mui/material";

import { FunctionIconGlyph } from "../functions/FunctionIconGlyph";
import {
  cockpit,
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
  const top = active ? cockpit.btnActiveTop : cockpit.btnTop;
  const bottom = active ? cockpit.btnActiveBottom : cockpit.btnBottom;
  const text = `${fnCode}: ${label}`;

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
        border: `1px solid ${active ? cockpit.borderBright : cockpit.border}`,
        borderRadius: 1,
        cursor: disabled ? "not-allowed" : "pointer",
        background: `linear-gradient(145deg, ${top} 0%, ${bottom} 100%)`,
        boxShadow: active
          ? "inset 0 1px 4px rgba(255,255,255,0.18), 0 1px 3px rgba(0,0,0,0.35)"
          : "inset 0 1px 0 rgba(255,255,255,0.12), 0 1px 2px rgba(0,0,0,0.3)",
        opacity: disabled ? 0.45 : 1,
        textAlign: "left",
        overflow: "hidden",
      }}
    >
      <FunctionIconGlyph
        slug={icon}
        size={FUNCTION_LIST_ICON_PX}
        variant="cockpit"
        active={active}
      />
      <Typography
        component="span"
        sx={{
          flex: 1,
          minWidth: 0,
          color: cockpit.text,
          fontWeight: 600,
          fontSize: 13,
          lineHeight: 1.2,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          textShadow: "0 1px 0 rgba(0,0,0,0.85)",
        }}
      >
        {text}
      </Typography>
    </Box>
  );
}

export default memo(FunctionListRow);
