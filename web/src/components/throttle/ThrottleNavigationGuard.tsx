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
import { useBlocker } from "react-router-dom";

// ThrottleNavigationGuard blocks in-app navigation while the throttle
// lever is above zero and asks before leaving (emergency stop on confirm).
export default function ThrottleNavigationGuard({
  active,
  onLeaveConfirm,
}: {
  active: boolean;
  onLeaveConfirm: () => Promise<void>;
}) {
  const { t } = useTranslation("throttle");
  const blocker = useBlocker(active);
  const [busy, setBusy] = useState(false);
  const open = blocker.state === "blocked";

  const handleStay = () => {
    if (busy) {
      return;
    }
    blocker.reset?.();
  };

  const handleLeave = async () => {
    setBusy(true);
    try {
      await onLeaveConfirm();
      blocker.proceed?.();
    } catch {
      blocker.reset?.();
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onClose={handleStay}>
      <DialogTitle>{t("leaveGuard.title")}</DialogTitle>
      <DialogContent>
        <DialogContentText>{t("leaveGuard.message")}</DialogContentText>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2, gap: 1 }}>
        <Button
          onClick={handleStay}
          color="success"
          variant="contained"
          disabled={busy}
        >
          {t("leaveGuard.cancel")}
        </Button>
        <Button
          onClick={() => void handleLeave()}
          color="error"
          variant="contained"
          disabled={busy}
        >
          {t("leaveGuard.confirm")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
