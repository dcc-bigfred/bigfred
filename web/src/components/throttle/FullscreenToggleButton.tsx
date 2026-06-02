import { Button, Tooltip } from "@mui/material";
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
      <Button
        size="small"
        onClick={() => void toggle()}
        aria-label={label}
        startIcon={active ? <FullscreenExitIcon /> : <FullscreenIcon />}
        sx={{
          flexShrink: 0,
          color: cockpit.text,
          borderColor: cockpit.border,
          textTransform: "none",
          fontWeight: 500,
          whiteSpace: "nowrap",
          minWidth: 0,
          px: 1,
          "& .MuiButton-startIcon": { mr: 0.5 },
        }}
        variant="outlined"
      >
        {t("fullscreen.label")}
      </Button>
    </Tooltip>
  );
}
