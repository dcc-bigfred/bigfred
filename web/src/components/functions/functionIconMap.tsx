import { Box } from "@mui/material";

import {
  COCKPIT_FUNCTION_ICON_PX,
  FunctionIconGlyph,
  type FunctionIconSlug,
} from "./FunctionIconGlyph";

export type { FunctionIconSlug };

export function FunctionIconVisual({
  icon,
  size = 24,
  variant = "default",
  active = false,
}: {
  icon: string;
  size?: number;
  variant?: "default" | "cockpit";
  active?: boolean;
}) {
  return (
    <Box
      component="span"
      sx={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        lineHeight: 0,
        verticalAlign: "middle",
        color: variant === "cockpit" ? undefined : "primary.main",
      }}
    >
      <FunctionIconGlyph
        slug={icon}
        size={size}
        variant={variant}
        active={active}
      />
    </Box>
  );
}

export { COCKPIT_FUNCTION_ICON_PX, FunctionIconGlyph };
