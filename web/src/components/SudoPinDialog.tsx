import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { ApiError } from "../api/client";

// DialogTarget switches the localized title / description between
// the two PIN-gated flows. Both share the dialog component because
// the input affordances (numeric keypad, password masking, error
// display) are identical.
export type DialogTarget = "admin" | "signalman" | "promoteSignalman" | "demoteSignalman";

interface SudoPinDialogProps {
  open: boolean;
  target: DialogTarget;
  targetLogin?: string;
  onCancel: () => void;
  onSubmit: (pin: string) => Promise<void>;
}

// SudoPinDialog renders the modal that gates a sudo elevation
// (§7a.7). The PIN field is type="password" inputMode="numeric" so
// the on-screen keypad on a phone defaults to digits while the
// glyphs render as bullets.
//
// The dialog owns the PIN buffer so it can clear it on close — the
// buffer is never lifted to a parent component, never serialised,
// never logged.
export default function SudoPinDialog({
  open,
  target,
  targetLogin,
  onCancel,
  onSubmit,
}: SudoPinDialogProps) {
  const { t } = useTranslation(["sudo", "common", "errors"]);
  const [pin, setPin] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [errorCode, setErrorCode] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setPin("");
      setErrorCode(null);
      setSubmitting(false);
    }
  }, [open]);

  const handleSubmit = async () => {
    if (submitting || pin.length === 0) return;
    setSubmitting(true);
    setErrorCode(null);
    try {
      await onSubmit(pin);
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorCode(err.code);
      } else {
        setErrorCode("network");
      }
      setSubmitting(false);
      return;
    }
    setSubmitting(false);
  };

  return (
    <Dialog open={open} onClose={onCancel} fullWidth maxWidth="xs">
      <DialogTitle>{t(`sudo:dialog.title.${target}`)}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 2 }}>
          {t(`sudo:dialog.description.${target}`, {
            login: targetLogin ?? "",
          })}
        </DialogContentText>
        <TextField
          autoFocus
          fullWidth
          required
          label={t("sudo:dialog.pinLabel")}
          type="password"
          inputMode="numeric"
          autoComplete="off"
          value={pin}
          onChange={(e) => setPin(e.target.value.replace(/[^0-9]/g, "").slice(0, 8))}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              void handleSubmit();
            }
          }}
          slotProps={{
            input: {
              inputProps: {
                pattern: "[0-9]*",
                maxLength: 8,
                "aria-label": t("sudo:dialog.pinLabel"),
              },
            },
          }}
        />
        {errorCode && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {t(`errors:${errorCode}` as const, { defaultValue: t("errors:unknown", { code: errorCode }) })}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} disabled={submitting}>
          {t("sudo:dialog.cancel")}
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={submitting || pin.length < 4}
        >
          {t("sudo:dialog.submit")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
