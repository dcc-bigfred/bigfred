import { useState } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from "@mui/material";
import { useTranslation } from "react-i18next";

// SlotInUseDialog asks whether to steal a LocoNet slot held by another
// throttle (e.g. physical FRED) after the daemon reported slot_in_use.
export default function SlotInUseDialog({
  open,
  address,
  onDismiss,
  onSteal,
}: {
  open: boolean;
  address: number;
  onDismiss: () => void;
  onSteal: (address: number) => Promise<{ ok: boolean; error?: string }>;
}) {
  const { t } = useTranslation("throttle");
  const [busy, setBusy] = useState(false);

  const handleSteal = async () => {
    if (busy || address <= 0) {
      return;
    }
    setBusy(true);
    try {
      const res = await onSteal(address);
      if (res.ok) {
        onDismiss();
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onClose={busy ? undefined : onDismiss}>
      <DialogTitle>{t("slotInUse.title")}</DialogTitle>
      <DialogContent>
        <DialogContentText>{t("slotInUse.message")}</DialogContentText>
        <DialogContentText sx={{ mt: 1.5 }}>
          {t("slotInUse.hint")}
        </DialogContentText>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2, gap: 1 }}>
        <Button onClick={onDismiss} variant="outlined" disabled={busy}>
          {t("slotInUse.ok")}
        </Button>
        <Button
          onClick={() => void handleSteal()}
          color="warning"
          variant="contained"
          disabled={busy || address <= 0}
        >
          {t("slotInUse.steal")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
