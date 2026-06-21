import BackspaceOutlinedIcon from "@mui/icons-material/BackspaceOutlined";
import { Box, Button, Paper, Typography } from "@mui/material";
import { useTranslation } from "react-i18next";

export const USER_PIN_MAX_LENGTH = 12;

/** Keeps only digits and caps length for user PIN fields. */
export function sanitizePinDigits(
  raw: string,
  maxLength = USER_PIN_MAX_LENGTH,
): string {
  return raw.replace(/\D/g, "").slice(0, maxLength);
}

interface NumericKeypadProps {
  onDigit: (digit: string) => void;
  onBackspace: () => void;
  disabled?: boolean;
}

// NumericKeypad renders a touch-friendly 0–9 pad with backspace.
// Used under password fields on login and change-password screens so
// club-room tablets can enter a PIN without hunting for the OS keypad.
export default function NumericKeypad({
  onDigit,
  onBackspace,
  disabled = false,
}: NumericKeypadProps) {
  const { t } = useTranslation("common");

  const digitKeys = ["1", "2", "3", "4", "5", "6", "7", "8", "9"];

  return (
    <Paper
      variant="outlined"
      role="group"
      aria-label={t("keypad.groupLabel")}
      sx={{
        p: 1.5,
        bgcolor: "grey.50",
        borderColor: "divider",
      }}
    >
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "repeat(3, 1fr)",
          gap: 1,
          width: "100%",
          userSelect: "none",
        }}
      >
      {digitKeys.map((digit) => (
        <Button
          key={digit}
          type="button"
          variant="contained"
          color="inherit"
          disabled={disabled}
          onClick={() => onDigit(digit)}
          aria-label={t("keypad.digit", { digit })}
          sx={{
            minHeight: 56,
            fontSize: "1.35rem",
            fontWeight: 600,
            bgcolor: "background.paper",
            color: "text.primary",
            boxShadow: 1,
            "&:hover": { bgcolor: "grey.100" },
          }}
        >
          {digit}
        </Button>
      ))}
      <Button
        type="button"
        variant="contained"
        color="inherit"
        disabled={disabled}
        onClick={onBackspace}
        aria-label={t("keypad.backspace")}
        sx={{
          minHeight: 56,
          bgcolor: "background.paper",
          color: "text.primary",
          boxShadow: 1,
          "&:hover": { bgcolor: "grey.100" },
        }}
      >
        <BackspaceOutlinedIcon />
      </Button>
      <Button
        type="button"
        variant="contained"
        color="inherit"
        disabled={disabled}
        onClick={() => onDigit("0")}
        aria-label={t("keypad.digit", { digit: "0" })}
        sx={{
          minHeight: 56,
          fontSize: "1.35rem",
          fontWeight: 600,
          bgcolor: "background.paper",
          color: "text.primary",
          boxShadow: 1,
          "&:hover": { bgcolor: "grey.100" },
        }}
      >
        0
      </Button>
      <Box aria-hidden sx={{ minHeight: 56 }} />
      </Box>
    </Paper>
  );
}

/** Compact hint shown above the pad when a labelled field is active. */
export function NumericKeypadCaption({ label }: { label: string }) {
  return (
    <Typography
      variant="caption"
      color="text.secondary"
      textAlign="center"
      display="block"
      sx={{ mb: 1 }}
    >
      {label}
    </Typography>
  );
}
