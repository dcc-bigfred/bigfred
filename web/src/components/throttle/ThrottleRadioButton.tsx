import { useState } from "react";
import { IconButton, Tooltip } from "@mui/material";
import SettingsInputAntennaIcon from "@mui/icons-material/SettingsInputAntenna";
import { useTranslation } from "react-i18next";

import { cockpit } from "./throttleCockpitTheme";
import ThrottleRadioOverlay from "./ThrottleRadioOverlay";

interface ThrottleRadioButtonProps {
  layoutId: number;
  vehicleId: number | null;
  vehicleName: string | null;
}

// ThrottleRadioButton opens the interlocking picker + phrase dialog
// for the currently driven vehicle context.
export default function ThrottleRadioButton({
  layoutId,
  vehicleId,
  vehicleName,
}: ThrottleRadioButtonProps) {
  const { t } = useTranslation("throttle");
  const [open, setOpen] = useState(false);
  const disabled = vehicleId == null;

  return (
    <>
      <Tooltip title={t("radio.open")}>
        <span>
          <IconButton
            size="small"
            disabled={disabled}
            onClick={() => setOpen(true)}
            aria-label={t("radio.open")}
            sx={{ color: cockpit.text }}
          >
            <SettingsInputAntennaIcon fontSize="small" />
          </IconButton>
        </span>
      </Tooltip>
      {open && vehicleId != null && (
        <ThrottleRadioOverlay
          layoutId={layoutId}
          vehicleId={vehicleId}
          vehicleName={vehicleName ?? ""}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  );
}
