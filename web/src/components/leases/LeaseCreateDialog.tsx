import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Autocomplete,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { ApiError } from "../../api/client";
import {
  lendableTargetKey,
  useCreateLease,
  useLendable,
  type LendableTarget,
  type LendableUser,
} from "../../api/leases";
import type { TakeoverTarget } from "../../api/takeover";
import { getUserName } from "../../utils/getUserName";

export interface LeaseCreateDialogProps {
  open: boolean;
  onClose: () => void;
  initialTarget?: { kind: TakeoverTarget; targetId: string; targetName?: string } | null;
  allowUnresolvedTarget?: boolean;
}

export default function LeaseCreateDialog({
  open,
  onClose,
  initialTarget = null,
  allowUnresolvedTarget = false,
}: LeaseCreateDialogProps) {
  const { t } = useTranslation(["rentals", "common", "errors"]);
  const lendable = useLendable();
  const create = useCreateLease();

  const [selectedTargetKey, setSelectedTargetKey] = useState("");
  const [selectedUser, setSelectedUser] = useState<LendableUser | null>(null);
  const [speedLimit, setSpeedLimit] = useState("80");
  const [hours, setHours] = useState("0");
  const [minutes, setMinutes] = useState("30");
  const [unresolvedTargetName, setUnresolvedTargetName] = useState<string | null>(null);

  const targets = lendable.data?.targets ?? [];
  const users = lendable.data?.users ?? [];

  useEffect(() => {
    if (open) {
      create.reset();
      setSelectedTargetKey(
        initialTarget ? lendableTargetKey(initialTarget) : "",
      );
      setUnresolvedTargetName(initialTarget?.targetName ?? null);
      setSelectedUser(null);
      setSpeedLimit("80");
      setHours("0");
      setMinutes("30");
    }
    // create.reset is stable; full `create` would retrigger every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initialTarget]);

  const selectedTarget = targets.find(
    (tgt) => lendableTargetKey(tgt) === selectedTargetKey,
  );

  const submitTarget = useMemo((): LendableTarget | null => {
    if (selectedTarget) {
      return selectedTarget;
    }
    if (allowUnresolvedTarget) {
      const parsed = parseLendableTargetKey(selectedTargetKey);
      if (!parsed) {
        return null;
      }
      if (unresolvedTargetName) {
        return { ...parsed, targetName: unresolvedTargetName };
      }
      return parsed;
    }
    return null;
  }, [selectedTarget, selectedTargetKey, allowUnresolvedTarget, unresolvedTargetName]);

  const submitError = (() => {
    const err = create.error;
    if (!err) return null;
    if (err instanceof ApiError) {
      const key = `errors:${err.code}` as const;
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  })();

  const handleClose = () => {
    if (!create.isPending) {
      onClose();
    }
  };

  const handleSubmit = async () => {
    if (!submitTarget || !selectedUser) return;
    const h = Number(hours) || 0;
    const m = Number(minutes) || 0;
    const durationSeconds = h * 3600 + m * 60;
    const limit = Number(speedLimit);
    try {
      await create.mutateAsync({
        kind: submitTarget.kind,
        targetId: submitTarget.targetId,
        toUserId: selectedUser.userId,
        speedLimit: Number.isFinite(limit) ? Math.min(100, Math.max(0, limit)) : 0,
        durationSeconds,
      });
      onClose();
    } catch {
      // surfaced via submitError
    }
  };

  return (
    <Dialog open={open} onClose={handleClose} fullWidth maxWidth="sm">
      <DialogTitle>{t("create.title")}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          {submitError && <Alert severity="error">{submitError}</Alert>}
          <FormControl fullWidth>
            <InputLabel id="lease-target-label">{t("create.target")}</InputLabel>
            <Select
              labelId="lease-target-label"
              label={t("create.target")}
              value={selectedTargetKey}
              onChange={(e) => setSelectedTargetKey(e.target.value)}
            >
              {allowUnresolvedTarget && selectedTargetKey && !selectedTarget && (
                <MenuItem value={selectedTargetKey}>
                  {submitTarget?.targetName ?? selectedTargetKey}
                </MenuItem>
              )}
              {targets.map((tgt) => (
                <MenuItem key={lendableTargetKey(tgt)} value={lendableTargetKey(tgt)}>
                  {tgt.targetName} ({tgt.kind === "train" ? t("kind.train") : t("kind.vehicle")})
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <Autocomplete
            options={users}
            getOptionLabel={(u) => getUserName(u)}
            value={selectedUser}
            onChange={(_event, value) => setSelectedUser(value)}
            isOptionEqualToValue={(a, b) => a.userId === b.userId}
            loading={lendable.isLoading}
            renderInput={(params) => (
              <TextField {...params} label={t("create.user")} />
            )}
          />

          <TextField
            label={t("create.speedLimit")}
            type="number"
            inputProps={{ min: 0, max: 100 }}
            value={speedLimit}
            onChange={(e) => setSpeedLimit(e.target.value)}
            helperText={t("create.speedLimitHelp")}
          />

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
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={create.isPending}>
          {t("common:actions.cancel")}
        </Button>
        <Button
          variant="contained"
          onClick={() => void handleSubmit()}
          disabled={
            create.isPending ||
            !submitTarget ||
            !selectedUser ||
            (Number(hours) === 0 && Number(minutes) === 0)
          }
        >
          {t("create.submit")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function parseLendableTargetKey(key: string): LendableTarget | null {
  const sep = key.indexOf(":");
  if (sep <= 0) {
    return null;
  }
  const kind = key.slice(0, sep) as TakeoverTarget;
  if (kind !== "vehicle" && kind !== "train") {
    return null;
  }
  const targetId = key.slice(sep + 1);
  if (!targetId) {
    return null;
  }
  return { kind, targetId, targetName: targetId };
}
