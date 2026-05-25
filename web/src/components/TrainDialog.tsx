import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  MenuItem,
  Stack,
  Switch,
  TextField,
  Typography,
} from "@mui/material";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import ArrowDownwardIcon from "@mui/icons-material/ArrowDownward";
import DeleteIcon from "@mui/icons-material/Delete";
import AddIcon from "@mui/icons-material/Add";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";
import {
  useCreateTrain,
  useMyVehicles,
  useUpdateTrain,
  type Train,
  type TrainMemberInput,
} from "../api/vehicles";

interface Props {
  open: boolean;
  train?: Train | null;
  onClose: () => void;
}

// TrainDialog hosts the create/edit form for a single train. Members
// are an ordered list rendered as a draggable-style table with
// up/down arrow buttons + a "reversed" toggle.
export default function TrainDialog({ open, train, onClose }: Props) {
  const { t } = useTranslation(["vehicle", "errors", "common"]);
  const isEdit = !!train;

  const [name, setName] = useState("");
  const [members, setMembers] = useState<TrainMemberInput[]>([]);
  const [picker, setPicker] = useState<number | "">("");

  const create = useCreateTrain();
  const update = useUpdateTrain();
  const vehicles = useMyVehicles();

  useEffect(() => {
    if (!open) return;
    if (train) {
      setName(train.name);
      setMembers(
        train.members.map((m) => ({
          vehicleId: m.vehicleId,
          reversed: m.reversed,
        })),
      );
    } else {
      setName("");
      setMembers([]);
    }
    setPicker("");
    create.reset();
    update.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, train?.id]);

  const availableForPicker = useMemo(() => {
    if (!vehicles.data) return [];
    const taken = new Set(members.map((m) => m.vehicleId));
    return vehicles.data.filter((v) => !taken.has(v.id));
  }, [vehicles.data, members]);

  const labelFor = (vehicleId: number) => {
    const v = vehicles.data?.find((x) => x.id === vehicleId);
    if (!v) return `#${vehicleId}`;
    const dcc = v.dccAddress != null ? ` · DCC ${v.dccAddress}` : "";
    return `${v.name}${dcc}`;
  };

  const onAddMember = () => {
    if (picker === "") return;
    setMembers((prev) => [...prev, { vehicleId: Number(picker), reversed: false }]);
    setPicker("");
  };

  const onRemoveMember = (idx: number) => {
    setMembers((prev) => prev.filter((_, i) => i !== idx));
  };

  const onMove = (idx: number, delta: number) => {
    setMembers((prev) => {
      const next = [...prev];
      const target = idx + delta;
      if (target < 0 || target >= next.length) return prev;
      [next[idx], next[target]] = [next[target], next[idx]];
      return next;
    });
  };

  const onToggleReversed = (idx: number, value: boolean) => {
    setMembers((prev) =>
      prev.map((m, i) => (i === idx ? { ...m, reversed: value } : m)),
    );
  };

  const onSubmit = () => {
    if (isEdit && train) {
      update.mutate({ id: train.id, name, members });
    } else {
      create.mutate({ name, members });
    }
  };

  useEffect(() => {
    if (create.isSuccess || update.isSuccess) onClose();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [create.isSuccess, update.isSuccess]);

  const errorMessage = (() => {
    const err = create.error ?? update.error;
    if (!err) return null;
    if (err instanceof ApiError) {
      const key = `errors:${err.code}` as const;
      const translated = t(key, { defaultValue: "" });
      if (translated) return translated;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  })();

  const submitting = create.isPending || update.isPending;
  const canSubmit = name.trim().length > 0 && members.length > 0 && !submitting;
  const noVehicles = vehicles.data && vehicles.data.length === 0;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {isEdit
          ? t("vehicle:trainDialog.edit.title")
          : t("vehicle:trainDialog.create.title")}
      </DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <TextField
            label={t("vehicle:trainDialog.fields.name")}
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
            fullWidth
            required
          />

          <Box>
            <Typography variant="subtitle2" gutterBottom>
              {t("vehicle:trainDialog.fields.members")}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {t("vehicle:trainDialog.fields.membersHelp")}
            </Typography>
          </Box>

          {noVehicles && (
            <Alert severity="info">{t("vehicle:trainDialog.noVehicles")}</Alert>
          )}

          <Stack spacing={1}>
            {members.map((m, idx) => (
              <Stack
                key={`${m.vehicleId}-${idx}`}
                direction="row"
                spacing={1}
                alignItems="center"
                sx={{
                  px: 1.5,
                  py: 1,
                  borderRadius: 1,
                  border: 1,
                  borderColor: "divider",
                  bgcolor: "background.paper",
                }}
              >
                <Typography variant="body2" sx={{ flexGrow: 1 }}>
                  {labelFor(m.vehicleId)}
                </Typography>
                <Switch
                  size="small"
                  checked={m.reversed}
                  onChange={(e) => onToggleReversed(idx, e.target.checked)}
                  inputProps={{ "aria-label": t("vehicle:trainDialog.reversed") }}
                />
                <Typography variant="caption" color="text.secondary">
                  {t("vehicle:trainDialog.reversed")}
                </Typography>
                <IconButton
                  size="small"
                  onClick={() => onMove(idx, -1)}
                  disabled={idx === 0}
                  aria-label={t("vehicle:trainDialog.movePrev")}
                >
                  <ArrowUpwardIcon fontSize="small" />
                </IconButton>
                <IconButton
                  size="small"
                  onClick={() => onMove(idx, 1)}
                  disabled={idx === members.length - 1}
                  aria-label={t("vehicle:trainDialog.moveNext")}
                >
                  <ArrowDownwardIcon fontSize="small" />
                </IconButton>
                <IconButton
                  size="small"
                  onClick={() => onRemoveMember(idx)}
                  aria-label={t("vehicle:trainDialog.removeMember")}
                >
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Stack>
            ))}
          </Stack>

          {!noVehicles && (
            <Stack direction="row" spacing={1}>
              <TextField
                select
                value={picker}
                onChange={(e) =>
                  setPicker(e.target.value === "" ? "" : Number(e.target.value))
                }
                label={t("vehicle:trainDialog.addMember")}
                size="small"
                sx={{ flexGrow: 1 }}
                disabled={availableForPicker.length === 0}
              >
                {availableForPicker.map((v) => (
                  <MenuItem key={v.id} value={String(v.id)}>
                    {labelFor(v.id)}
                  </MenuItem>
                ))}
              </TextField>
              <Button
                variant="outlined"
                onClick={onAddMember}
                disabled={picker === ""}
                startIcon={<AddIcon />}
              >
                {t("vehicle:trainDialog.addMember")}
              </Button>
            </Stack>
          )}

          {errorMessage && <Alert severity="error">{errorMessage}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t("vehicle:trainDialog.cancel")}</Button>
        <Button onClick={onSubmit} disabled={!canSubmit} variant="contained">
          {isEdit
            ? t("vehicle:trainDialog.submitEdit")
            : t("vehicle:trainDialog.submitCreate")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
