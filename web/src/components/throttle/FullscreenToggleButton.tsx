import { IconButton, Tooltip } from "@mui/material";
import FullscreenIcon from "@mui/icons-material/Fullscreen";
import FullscreenExitIcon from "@mui/icons-material/FullscreenExit";
import { useTranslation } from "react-i18next";

import { useFullscreen } from "../../hooks/useFullscreen";
import { cockpit } from "./throttleCockpitTheme";

// FullscreenToggleButton requests browser fullscreen for the throttle view.
export default function FullscreenToggleButton() {
  const { t } = useTranslation("throttle");
  const { active, toggle, supported } = useFullscreen();

  if (!supported) {
    return null;
  }

  const label = active ? t("fullscreen.exit") : t("fullscreen.enter");

  return (
    <Tooltip title={label}>
      <IconButton
        size="small"
        onClick={() => void toggle()}
        aria-label={label}
        sx={{ color: cockpit.text }}
      >
        {active ? <FullscreenExitIcon fontSize="small" /> : <FullscreenIcon fontSize="small" />}
      </IconButton>
    </Tooltip>
  );
}
