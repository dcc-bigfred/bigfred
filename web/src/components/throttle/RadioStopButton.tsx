import { useCallback, useMemo, useState } from "react";
import {
  Button,
  Dialog,
  IconButton,
  Stack,
  Tooltip,
} from "@mui/material";
import SettingsInputAntennaIcon from "@mui/icons-material/SettingsInputAntenna";
import { useTranslation } from "react-i18next";

import { useMe } from "../../api/auth";
import { useLayoutVehicles } from "../../api/vehicles";
import { useSocket } from "../../context/SocketContext";
import { cockpit } from "./throttleCockpitTheme";

interface RadioStopButtonProps {
  layoutId: number;
}

// RadioStopButton opens a confirmation overlay and fires system.radioStop
// on the control plane when the operator confirms (§4.6.3).
export default function RadioStopButton({ layoutId }: RadioStopButtonProps) {
  const { t } = useTranslation("throttle");
  const { sendAction } = useSocket();
  const me = useMe().data;
  const roster = useLayoutVehicles(layoutId).data ?? [];
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  const canTrigger = useMemo(() => {
    if (!me) {
      return false;
    }
    return roster.some(
      (v) =>
        v.dccAddress != null &&
        v.ownerId === me.id,
    );
  }, [me, roster]);

  const handleConfirm = useCallback(async () => {
    setBusy(true);
    try {
      await sendAction("system.radioStop", {});
      setOpen(false);
    } finally {
      setBusy(false);
    }
  }, [sendAction]);

  if (!canTrigger) {
    return null;
  }

  return (
    <>
      <Tooltip title={t("radioStop.tooltip")}>
        <IconButton
          size="small"
          color="error"
          onClick={() => setOpen(true)}
          aria-label={t("radioStop.button")}
          sx={{ flexShrink: 0 }}
        >
          <SettingsInputAntennaIcon fontSize="small" />
        </IconButton>
      </Tooltip>

      <Dialog
        open={open}
        onClose={() => !busy && setOpen(false)}
        aria-labelledby="radio-stop-title"
        PaperProps={{
          sx: {
            bgcolor: cockpit.header,
            border: `1px solid ${cockpit.border}`,
            minWidth: 280,
          },
        }}
      >
        <Stack spacing={2} sx={{ p: 3, alignItems: "stretch" }}>
          <Button
            variant="contained"
            color="error"
            size="large"
            disabled={busy}
            onClick={() => void handleConfirm()}
          >
            {t("radioStop.run")}
          </Button>
          <Button
            variant="text"
            disabled={busy}
            onClick={() => setOpen(false)}
            sx={{ color: cockpit.text }}
          >
            {t("radioStop.cancel")}
          </Button>
        </Stack>
      </Dialog>
    </>
  );
}
