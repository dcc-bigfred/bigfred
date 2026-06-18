import { useCallback, useEffect, useMemo, useState } from "react";
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
import { useRetryingSend } from "../../hooks/useRetryingSend";
import { cockpit } from "./throttleCockpitTheme";

interface RadioStopButtonProps {
  layoutId: number;
  variant?: "icon" | "bar";
  /** Fired when a radio-stop retry chain starts or finishes. */
  onRetryingChange?: (retrying: boolean) => void;
}

// RadioStopButton opens a confirmation overlay and fires system.radioStop
// on the control plane when the operator confirms (§4.6.3).
export default function RadioStopButton({
  layoutId,
  variant = "icon",
  onRetryingChange,
}: RadioStopButtonProps) {
  const { t } = useTranslation(["throttle", "interlocking"]);
  const { sendAction } = useSocket();
  const sendRadioStop = useCallback(
    () => sendAction("system.radioStop", {}),
    [sendAction],
  );
  const { dispatchAsync: sendRadioStopWithRetry, retrying } =
    useRetryingSend(sendRadioStop);
  const me = useMe().data;
  const roster = useLayoutVehicles(layoutId).data ?? [];
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  const canTrigger = useMemo(() => {
    if (!me) {
      return false;
    }
    if (variant === "bar") {
      return me.isSignalman;
    }
    return roster.some(
      (v) => v.dccAddress != null && v.ownerId === me.id,
    );
  }, [me, roster, variant]);

  const handleConfirm = useCallback(async () => {
    setBusy(true);
    try {
      const res = await sendRadioStopWithRetry();
      if (res.ok) {
        setOpen(false);
      }
    } finally {
      setBusy(false);
    }
  }, [sendRadioStopWithRetry]);

  useEffect(() => {
    onRetryingChange?.(retrying);
    return () => onRetryingChange?.(false);
  }, [onRetryingChange, retrying]);

  if (!canTrigger) {
    return null;
  }

  const trigger =
    variant === "bar" ? (
      <Button
        variant="contained"
        color="error"
        startIcon={<SettingsInputAntennaIcon />}
        onClick={() => setOpen(true)}
        aria-label={t("interlocking:view.radioStop.barLabel")}
        sx={{ alignSelf: "stretch" }}
      >
        {t("interlocking:view.radioStop.barLabel")}
      </Button>
    ) : (
      <Tooltip title={t("throttle:radioStop.tooltip")}>
        <IconButton
          size="small"
          color="error"
          onClick={() => setOpen(true)}
          aria-label={t("throttle:radioStop.button")}
          sx={{ flexShrink: 0 }}
        >
          <SettingsInputAntennaIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    );

  return (
    <>
      {trigger}

      <Dialog
        open={open}
        onClose={() => !busy && setOpen(false)}
        aria-labelledby="radio-stop-title"
        PaperProps={{
          sx: {
            bgcolor: variant === "icon" ? cockpit.header : undefined,
            border: variant === "icon" ? `1px solid ${cockpit.border}` : undefined,
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
            {t("throttle:radioStop.run")}
          </Button>
          <Button
            variant="text"
            disabled={busy}
            onClick={() => setOpen(false)}
            sx={variant === "icon" ? { color: cockpit.text } : undefined}
          >
            {t("throttle:radioStop.cancel")}
          </Button>
        </Stack>
      </Dialog>
    </>
  );
}
