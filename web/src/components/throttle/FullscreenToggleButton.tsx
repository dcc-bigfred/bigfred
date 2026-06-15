import { IconButton, Tooltip } from "@mui/material";
import FullscreenIcon from "@mui/icons-material/Fullscreen";
import FullscreenExitIcon from "@mui/icons-material/FullscreenExit";
import { useTranslation } from "react-i18next";

import { useFullscreen } from "../../hooks/useFullscreen";

interface FullscreenToggleButtonProps {
  /** MUI IconButton color; defaults to inherit for the app bar. */
  color?: "inherit" | "default";
  size?: "small" | "medium";
}

// FullscreenToggleButton requests browser fullscreen for the app shell.
export default function FullscreenToggleButton({
  color = "inherit",
  size = "medium",
}: FullscreenToggleButtonProps) {
  const { t } = useTranslation("throttle");
  const { active, toggle, supported } = useFullscreen();

  if (!supported) {
    return null;
  }

  const label = active ? t("fullscreen.exit") : t("fullscreen.enter");

  return (
    <Tooltip title={label}>
      <IconButton
        color={color}
        size={size}
        onClick={() => void toggle()}
        aria-label={label}
      >
        {active ? <FullscreenExitIcon /> : <FullscreenIcon />}
      </IconButton>
    </Tooltip>
  );
}
