import { useEffect, useState } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

const MIN_MULTIPLIER = 0.05;
const MAX_MULTIPLIER = 4.0;
const STEP = 0.05;

export interface TrainMemberSettingsDialogProps {
  open: boolean;
  memberName: string;
  isLeading: boolean;
  initialMultiplier: number;
  saving?: boolean;
  onClose: () => void;
  onSave: (multiplier: number) => void;
}

export default function TrainMemberSettingsDialog({
  open,
  memberName,
  isLeading,
  initialMultiplier,
  saving = false,
  onClose,
  onSave,
}: TrainMemberSettingsDialogProps) {
  const { t } = useTranslation(["throttle", "errors"]);
  const [value, setValue] = useState(String(initialMultiplier));

  useEffect(() => {
    if (open) {
      setValue(String(initialMultiplier));
    }
  }, [open, initialMultiplier]);

  const parsed = Number(value.replace(",", "."));
  const valid =
    Number.isFinite(parsed) && parsed >= MIN_MULTIPLIER && parsed <= MAX_MULTIPLIER;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>{t("throttle:train.multiplier.title")}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {memberName}
        </Typography>
        {isLeading ? (
          <Typography variant="body2">{t("throttle:train.multiplier.leadingFixed")}</Typography>
        ) : (
          <>
            <TextField
              fullWidth
              type="number"
              label={t("throttle:train.multiplier.field")}
              value={value}
              onChange={(ev) => setValue(ev.target.value)}
              inputProps={{ min: MIN_MULTIPLIER, max: MAX_MULTIPLIER, step: STEP }}
              helperText={t("throttle:train.multiplier.help")}
            />
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t("throttle:train.multiplier.cancel")}</Button>
        {!isLeading && (
          <Button
            variant="contained"
            disabled={!valid || saving}
            onClick={() => onSave(parsed)}
          >
            {t("throttle:train.multiplier.save")}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
