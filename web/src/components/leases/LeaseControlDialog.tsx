import { useEffect, useState } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import {
  useEstopTargetActions,
} from "../../api/estop";
import {
  useRevokeLease,
  useUpdateLease,
  type LeaseEntry,
} from "../../api/leases";
import LeaseCountdown from "./LeaseCountdown";

export interface LeaseControlDialogProps {
  lease: LeaseEntry | null;
  open: boolean;
  onClose: () => void;
}

export default function LeaseControlDialog({
  lease,
  open,
  onClose,
}: LeaseControlDialogProps) {
  const { t } = useTranslation(["rentals", "common"]);
  const update = useUpdateLease();
  const revoke = useRevokeLease();
  const { estopTarget } = useEstopTargetActions();

  const [speedLimit, setSpeedLimit] = useState("");
  const [hours, setHours] = useState("0");
  const [minutes, setMinutes] = useState("30");

  useEffect(() => {
    if (lease && open) {
      setSpeedLimit(String(lease.speedLimit));
      setHours("0");
      setMinutes("30");
    }
  }, [lease, open]);

  const handleClose = () => {
    if (!update.isPending && !revoke.isPending) {
      onClose();
    }
  };

  if (!lease) {
    return null;
  }

  const applySpeed = async () => {
    const limit = Number(speedLimit);
    await update.mutateAsync({
      kind: lease.kind,
      targetId: lease.targetId,
      speedLimit: Number.isFinite(limit) ? Math.min(100, Math.max(0, limit)) : 0,
    });
  };

  const applyDuration = async () => {
    const h = Number(hours) || 0;
    const m = Number(minutes) || 0;
    await update.mutateAsync({
      kind: lease.kind,
      targetId: lease.targetId,
      durationSeconds: h * 3600 + m * 60,
    });
  };

  const stop = async () => {
    await estopTarget(lease.kind, lease.targetId);
  };

  const returnLease = async () => {
    await revoke.mutateAsync({ kind: lease.kind, targetId: lease.targetId });
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleClose} fullWidth maxWidth="sm">
      <DialogTitle>{lease.targetName}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <Typography variant="body2" color="text.secondary">
            {t("control.lessee")}: {lease.toLogin}
          </Typography>
          <Typography variant="body2">
            {t("control.remaining")}:{" "}
            <LeaseCountdown expiresAt={lease.expiresAt} expiredLabel={t("expired")} />
          </Typography>

          <Stack direction="row" spacing={1} alignItems="flex-end">
            <TextField
              label={t("control.speedLimit")}
              type="number"
              inputProps={{ min: 0, max: 100 }}
              value={speedLimit}
              onChange={(e) => setSpeedLimit(e.target.value)}
              fullWidth
            />
            <Button variant="outlined" onClick={() => void applySpeed()} disabled={update.isPending}>
              {t("control.apply")}
            </Button>
          </Stack>

          <Button variant="contained" color="warning" onClick={() => void stop()}>
            {t("control.stop")}
          </Button>

          <Button variant="outlined" color="error" onClick={() => void returnLease()} disabled={revoke.isPending}>
            {t("control.return")}
          </Button>

          <Typography variant="subtitle2">{t("control.adjustTime")}</Typography>
          <Stack direction="row" spacing={2}>
            <TextField
              label={t("create.hours")}
              type="number"
              inputProps={{ min: 0, max: 24 }}
              value={hours}
              onChange={(e) => setHours(e.target.value)}
              fullWidth
            />
            <TextField
              label={t("create.minutes")}
              type="number"
              inputProps={{ min: 0, max: 59 }}
              value={minutes}
              onChange={(e) => setMinutes(e.target.value)}
              fullWidth
            />
          </Stack>
          <Button variant="outlined" onClick={() => void applyDuration()} disabled={update.isPending}>
            {t("control.adjustTimeSubmit")}
          </Button>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>{t("common:actions.close")}</Button>
      </DialogActions>
    </Dialog>
  );
}
