import { useCallback, useEffect, useState } from "react";
import {
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import {
  fetchSystemInfo,
  requestSystemShutdown,
  type SystemShutdownMode,
} from "../api/system";
import { ApiError } from "../api/client";

type Phase = "loading" | "unavailable" | "confirm" | "working" | "done";

/**
 * Admin dialog for host poweroff/reboot when microinit runs in init mode
 * (BigFredOS). POST is fire-and-forget — see plan D2.1.
 */
export default function SystemPowerDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation("common");
  const [phase, setPhase] = useState<Phase>("loading");
  const [workingMode, setWorkingMode] = useState<SystemShutdownMode | null>(
    null,
  );

  useEffect(() => {
    if (!open) {
      setPhase("loading");
      setWorkingMode(null);
      return;
    }
    let cancelled = false;
    setPhase("loading");
    void fetchSystemInfo()
      .then((info) => {
        if (!cancelled) {
          setPhase(info.canShutdown ? "confirm" : "unavailable");
        }
      })
      .catch(() => {
        if (!cancelled) setPhase("unavailable");
      });
    return () => {
      cancelled = true;
    };
  }, [open]);

  const handleClose = useCallback(() => {
    if (phase === "working" || phase === "done") return;
    onClose();
  }, [onClose, phase]);

  const run = async (mode: SystemShutdownMode) => {
    setWorkingMode(mode);
    setPhase("working");
    try {
      await requestSystemShutdown(mode);
      setPhase("done");
    } catch (err) {
      if (err instanceof ApiError && err.code === "system_not_init") {
        setPhase("unavailable");
        return;
      }
      setPhase("done");
    }
  };

  const title =
    phase === "unavailable"
      ? t("systemPower.unavailableTitle")
      : phase === "done" || phase === "working"
        ? workingMode === "reboot"
          ? t("systemPower.restartingTitle")
          : t("systemPower.shuttingDownTitle")
        : t("systemPower.title");

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        {phase === "loading" || phase === "working" ? (
          <CircularProgress size={32} sx={{ display: "block", mx: "auto", my: 2 }} />
        ) : null}
        {phase === "unavailable" ? (
          <DialogContentText>{t("systemPower.unavailableBody")}</DialogContentText>
        ) : null}
        {phase === "confirm" ? (
          <DialogContentText>{t("systemPower.confirmBody")}</DialogContentText>
        ) : null}
        {phase === "done" || phase === "working" ? (
          <DialogContentText sx={{ mt: phase === "working" ? 2 : 0, textAlign: phase === "working" ? "center" : "left" }}>
            {workingMode === "reboot"
              ? t("systemPower.restartingBody")
              : t("systemPower.shuttingDownBody")}
          </DialogContentText>
        ) : null}
      </DialogContent>
      <DialogActions>
        {phase === "unavailable" ? (
          <Button onClick={onClose}>{t("actions.close")}</Button>
        ) : null}
        {phase === "confirm" ? (
          <>
            <Button onClick={onClose}>{t("actions.cancel")}</Button>
            <Button color="error" variant="contained" onClick={() => void run("poweroff")}>
              {t("systemPower.shutdown")}
            </Button>
            <Button color="warning" variant="contained" onClick={() => void run("reboot")}>
              {t("systemPower.restart")}
            </Button>
          </>
        ) : null}
      </DialogActions>
    </Dialog>
  );
}
