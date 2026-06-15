import { useMemo, useState } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import {
  radioPhrasesForSide,
  useSendRadio,
  type RadioPhrase,
  type RadioPhraseSide,
  type RadioSendContext,
  type RadioSendTarget,
} from "../../api/radio";
import { useRadioSounds } from "../../hooks/useRadioSounds";

export interface RadioPhrasePickerDialogProps {
  open: boolean;
  onClose: () => void;
  to: RadioSendTarget;
  context: RadioSendContext;
  side: RadioPhraseSide;
  targetLabel?: string;
  contextLabel?: string;
}

// RadioPhrasePickerDialog lists the closed phrase vocabulary with a
// client-side filter and emits radio.send on confirm.
export default function RadioPhrasePickerDialog({
  open,
  onClose,
  to,
  context,
  side,
  targetLabel,
  contextLabel,
}: RadioPhrasePickerDialogProps) {
  const { t } = useTranslation(["radio", "errors"]);
  const sendRadio = useSendRadio();
  const { playSent } = useRadioSounds(false);
  const [query, setQuery] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const vocabulary = radioPhrasesForSide(side);

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return vocabulary.filter((phrase) => {
      const label = t(`radio:phrase.${phrase}`).toLowerCase();
      return !q || label.includes(q) || phrase.toLowerCase().includes(q);
    });
  }, [query, t, vocabulary]);

  const handleSend = async (phrase: RadioPhrase) => {
    setBusy(true);
    setError(null);
    try {
      const result = await sendRadio({ to, context, phrase, note: note.trim() });
      if (!result.ok) {
        setError(
          t(`errors:${result.error}` as const, {
            defaultValue: result.error ?? t("errors:unknown", { code: "send_failed" }),
          }),
        );
        return;
      }
      playSent();
      setQuery("");
      setNote("");
      onClose();
    } finally {
      setBusy(false);
    }
  };

  const handleClose = () => {
    if (busy) return;
    setError(null);
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t("radio:picker.title")}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 0.5 }}>
          {(targetLabel || contextLabel) && (
            <Typography variant="body2" color="text.secondary">
              {[targetLabel, contextLabel].filter(Boolean).join(" · ")}
            </Typography>
          )}
          <TextField
            size="small"
            fullWidth
            autoFocus
            placeholder={t("radio:picker.search")}
            value={query}
            onChange={(ev) => setQuery(ev.target.value)}
          />
          <TextField
            size="small"
            fullWidth
            placeholder={t("radio:picker.note")}
            value={note}
            onChange={(ev) => setNote(ev.target.value)}
            inputProps={{ maxLength: 80 }}
          />
          {error && (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          )}
          <TableContainer sx={{ maxHeight: 320 }}>
            <Table size="small" stickyHeader>
              <TableHead>
                <TableRow>
                  <TableCell>{t("radio:picker.search")}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {rows.map((phrase) => (
                  <TableRow
                    key={phrase}
                    hover
                    sx={{ cursor: busy ? "default" : "pointer" }}
                    onClick={() => {
                      if (!busy) void handleSend(phrase);
                    }}
                  >
                    <TableCell>{t(`radio:phrase.${phrase}`)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={busy}>
          {t("radio:picker.cancel")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
