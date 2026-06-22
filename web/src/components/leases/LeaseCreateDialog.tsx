import { useEffect, useState } from "react";
import {
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

import {
  useCreateLease,
  useLendable,
  type LendableTarget,
  type LendableUser,
} from "../../api/leases";
import { getUserName } from "../../utils/getUserName";

export interface LeaseCreateDialogProps {
  open: boolean;
  onClose: () => void;
}

export default function LeaseCreateDialog({ open, onClose }: LeaseCreateDialogProps) {
  const { t } = useTranslation(["rentals", "common"]);
  const lendable = useLendable();
  const create = useCreateLease();

  const [selectedTargetKey, setSelectedTargetKey] = useState("");
  const [selectedUser, setSelectedUser] = useState<LendableUser | null>(null);
  const [speedLimit, setSpeedLimit] = useState("80");
  const [hours, setHours] = useState("0");
  const [minutes, setMinutes] = useState("30");

  const targets = lendable.data?.targets ?? [];
  const users = lendable.data?.users ?? [];

  useEffect(() => {
    if (open) {
      setSelectedTargetKey("");
      setSelectedUser(null);
      setSpeedLimit("80");
      setHours("0");
      setMinutes("30");
    }
  }, [open]);

  const selectedTarget = targets.find(
    (tgt) => lendableTargetKey(tgt) === selectedTargetKey,
  );

  const handleClose = () => {
    if (!create.isPending) {
      onClose();
    }
  };

  const handleSubmit = async () => {
    if (!selectedTarget || !selectedUser) return;
    const h = Number(hours) || 0;
    const m = Number(minutes) || 0;
    const durationSeconds = h * 3600 + m * 60;
    const limit = Number(speedLimit);
    await create.mutateAsync({
      kind: selectedTarget.kind,
      targetId: selectedTarget.targetId,
      toUserId: selectedUser.userId,
      speedLimit: Number.isFinite(limit) ? Math.min(100, Math.max(0, limit)) : 0,
      durationSeconds,
    });
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleClose} fullWidth maxWidth="sm">
      <DialogTitle>{t("create.title")}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <FormControl fullWidth>
            <InputLabel id="lease-target-label">{t("create.target")}</InputLabel>
            <Select
              labelId="lease-target-label"
              label={t("create.target")}
              value={selectedTargetKey}
              onChange={(e) => setSelectedTargetKey(e.target.value)}
            >
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
            !selectedTarget ||
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

function lendableTargetKey(tgt: LendableTarget): string {
  return `${tgt.kind}:${tgt.targetId}`;
}
