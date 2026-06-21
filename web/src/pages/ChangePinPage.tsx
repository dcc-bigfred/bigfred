import { useState } from "react";
import {
  Alert,
  Box,
  Button,
  Container,
  Paper,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { useTranslation } from "react-i18next";
import { Link as RouterLink, useNavigate } from "react-router-dom";

import { useChangePin } from "../api/auth";
import { ApiError } from "../api/client";
import NumericKeypad, {
  NumericKeypadCaption,
  sanitizePinDigits,
} from "../components/NumericKeypad";

const PIN_PATTERN = /^\d{4,12}$/;

type PinField = "current" | "new" | "confirm";

export default function ChangePinPage() {
  const { t } = useTranslation(["user", "common", "errors"]);
  const navigate = useNavigate();
  const changePin = useChangePin();

  const [currentPin, setCurrentPin] = useState("");
  const [newPin, setNewPin] = useState("");
  const [confirmPin, setConfirmPin] = useState("");
  const [activeField, setActiveField] = useState<PinField>("current");

  const currentValid = currentPin.length > 0;
  const newValid = PIN_PATTERN.test(newPin);
  const confirmValid = confirmPin === newPin && newValid;
  const canSubmit =
    currentValid && newValid && confirmValid && !changePin.isPending;

  const translateError = (err: unknown): string => {
    if (err instanceof ApiError) {
      const localised = t(`errors:${err.code}` as const, { defaultValue: "" });
      if (localised) return localised;
      return t("errors:unknown", { code: err.code });
    }
    return t("errors:network");
  };

  const pinFieldLabels: Record<PinField, string> = {
    current: t("user:changePin.fields.currentPin"),
    new: t("user:changePin.fields.newPin"),
    confirm: t("user:changePin.fields.confirmPin"),
  };

  const setActivePin = {
    current: setCurrentPin,
    new: setNewPin,
    confirm: setConfirmPin,
  }[activeField];

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    changePin.mutate(
      { currentPin, newPin },
      {
        onSuccess: () => {
          navigate("/account/profile", {
            replace: true,
            state: { pinChanged: true },
          });
        },
      },
    );
  };

  return (
    <Container maxWidth="sm" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Box>
          <Button
            component={RouterLink}
            to="/account/profile"
            startIcon={<ArrowBackIcon />}
            sx={{ mb: 2 }}
          >
            {t("common:actions.back")}
          </Button>
          <Typography variant="h4" component="h1" gutterBottom>
            {t("user:changePin.title")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("user:changePin.subtitle")}
          </Typography>
        </Box>

        <Paper variant="outlined" sx={{ p: { xs: 2, sm: 3 } }}>
          <Box component="form" onSubmit={onSubmit} noValidate>
            <Stack spacing={2.5}>
              {changePin.error && (
                <Alert severity="error">{translateError(changePin.error)}</Alert>
              )}

              <TextField
                label={t("user:changePin.fields.currentPin")}
                type="password"
                inputMode="numeric"
                autoComplete="current-password"
                value={currentPin}
                onFocus={() => setActiveField("current")}
                onChange={(e) =>
                  setCurrentPin(sanitizePinDigits(e.target.value))
                }
                fullWidth
                required
              />

              <TextField
                label={t("user:changePin.fields.newPin")}
                type="password"
                inputMode="numeric"
                autoComplete="new-password"
                value={newPin}
                onFocus={() => setActiveField("new")}
                onChange={(e) => setNewPin(sanitizePinDigits(e.target.value))}
                helperText={t("user:admin.dialogs.fields.pinCreateHelp")}
                error={newPin.length > 0 && !newValid}
                fullWidth
                required
              />

              <TextField
                label={t("user:changePin.fields.confirmPin")}
                type="password"
                inputMode="numeric"
                autoComplete="new-password"
                value={confirmPin}
                onFocus={() => setActiveField("confirm")}
                onChange={(e) =>
                  setConfirmPin(sanitizePinDigits(e.target.value))
                }
                error={confirmPin.length > 0 && confirmPin !== newPin}
                helperText={
                  confirmPin.length > 0 && confirmPin !== newPin
                    ? t("user:changePin.fields.confirmMismatch")
                    : undefined
                }
                fullWidth
                required
              />

              <Box sx={{ pt: 0.5 }}>
                <NumericKeypadCaption
                  label={t("common:keypad.activeField", {
                    label: pinFieldLabels[activeField],
                  })}
                />
                <NumericKeypad
                  disabled={changePin.isPending}
                  onDigit={(digit) =>
                    setActivePin((prev) => sanitizePinDigits(prev + digit))
                  }
                  onBackspace={() => setActivePin((prev) => prev.slice(0, -1))}
                />
              </Box>

              <Stack direction="row" spacing={2} justifyContent="flex-end">
                <Button component={RouterLink} to="/account/profile">
                  {t("common:actions.cancel")}
                </Button>
                <Button type="submit" variant="contained" disabled={!canSubmit}>
                  {t("user:changePin.submit")}
                </Button>
              </Stack>
            </Stack>
          </Box>
        </Paper>
      </Stack>
    </Container>
  );
}
