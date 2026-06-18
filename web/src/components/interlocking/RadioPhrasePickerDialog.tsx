import { Fragment, useEffect, useMemo, useState } from "react";
import CloseIcon from "@mui/icons-material/Close";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  TextField,
  Typography,
} from "@mui/material";
import type { SelectChangeEvent } from "@mui/material";
import { useTranslation } from "react-i18next";

import {
  RADIO_PHRASE_GROUP_ALL,
  RADIO_PHRASE_GROUP_ORDER,
  radioPhraseGroup,
  radioPhraseIsContextualConfirmation,
  radioPhrasesForSide,
  readStoredRadioPhraseGroupFilter,
  sortRadioPhrasesWithinGroup,
  useSendRadio,
  writeStoredRadioPhraseGroupFilter,
  type RadioPhrase,
  type RadioPhraseGroupFilter,
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

function PhraseRow({
  phrase,
  busy,
  label,
  contextualConfirmation,
  onSend,
}: {
  phrase: RadioPhrase;
  busy: boolean;
  label: string;
  contextualConfirmation: boolean;
  onSend: (phrase: RadioPhrase) => void;
}) {
  return (
    <TableRow
      hover
      sx={{ cursor: busy ? "default" : "pointer" }}
      onClick={() => {
        if (!busy) onSend(phrase);
      }}
    >
      <TableCell sx={contextualConfirmation ? { color: "success.dark" } : undefined}>
        {label}
      </TableCell>
    </TableRow>
  );
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
  const [groupFilter, setGroupFilter] = useState<RadioPhraseGroupFilter>(() =>
    readStoredRadioPhraseGroupFilter(side),
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const vocabulary = radioPhrasesForSide(side);

  useEffect(() => {
    if (open) {
      setGroupFilter(readStoredRadioPhraseGroupFilter(side));
    }
  }, [open, side]);

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return vocabulary.filter((phrase) => {
      if (groupFilter !== RADIO_PHRASE_GROUP_ALL && radioPhraseGroup(phrase, side) !== groupFilter) {
        return false;
      }
      const label = t(`radio:phrase.${phrase}`).toLowerCase();
      return !q || label.includes(q) || phrase.toLowerCase().includes(q);
    });
  }, [groupFilter, query, side, t, vocabulary]);

  const resetForm = () => {
    setQuery("");
    setNote("");
    setError(null);
  };

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
      resetForm();
      onClose();
    } finally {
      setBusy(false);
    }
  };

  const handleClose = () => {
    if (busy) return;
    resetForm();
    onClose();
  };

  const handleGroupChange = (ev: SelectChangeEvent<RadioPhraseGroupFilter>) => {
    const group = ev.target.value as RadioPhraseGroupFilter;
    setGroupFilter(group);
    writeStoredRadioPhraseGroupFilter(side, group);
  };

  const phraseLabel = (phrase: RadioPhrase) => t(`radio:phrase.${phrase}`);
  const isContextualConfirmation = (phrase: RadioPhrase) =>
    radioPhraseIsContextualConfirmation(phrase, side);

  const sortedGroupRows = (group: (typeof RADIO_PHRASE_GROUP_ORDER)[number], phrases: RadioPhrase[]) =>
    sortRadioPhrasesWithinGroup(phrases, side, group);

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          pr: 1,
        }}
      >
        <Typography variant="h6" component="span">
          {t("radio:picker.title")}
        </Typography>
        <IconButton edge="end" onClick={handleClose} disabled={busy} aria-label={t("radio:picker.close")}>
          <CloseIcon />
        </IconButton>
      </DialogTitle>
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
          <FormControl size="small" fullWidth>
            <InputLabel id="radio-phrase-group-label">{t("radio:picker.groupLabel")}</InputLabel>
            <Select
              labelId="radio-phrase-group-label"
              label={t("radio:picker.groupLabel")}
              value={groupFilter}
              onChange={handleGroupChange}
            >
              <MenuItem value={RADIO_PHRASE_GROUP_ALL}>{t("radio:picker.group.all")}</MenuItem>
              {RADIO_PHRASE_GROUP_ORDER.map((group) => (
                <MenuItem key={group} value={group}>
                  {t(`radio:picker.group.${group}`)}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          {error && (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          )}
          <TableContainer sx={{ maxHeight: 320 }}>
            <Table size="small" stickyHeader>
              <TableBody>
                {groupFilter === RADIO_PHRASE_GROUP_ALL
                  ? RADIO_PHRASE_GROUP_ORDER.map((group) => {
                      const groupRows = sortedGroupRows(
                        group,
                        rows.filter((phrase) => radioPhraseGroup(phrase, side) === group),
                      );
                      if (groupRows.length === 0) {
                        return null;
                      }
                      return (
                        <Fragment key={group}>
                          <TableRow>
                            <TableCell
                              sx={{
                                fontWeight: 600,
                                bgcolor: "action.hover",
                                position: "sticky",
                                top: 0,
                                zIndex: 1,
                              }}
                            >
                              {t(`radio:picker.group.${group}`)}
                            </TableCell>
                          </TableRow>
                          {groupRows.map((phrase) => (
                            <PhraseRow
                              key={phrase}
                              phrase={phrase}
                              busy={busy}
                              label={phraseLabel(phrase)}
                              contextualConfirmation={isContextualConfirmation(phrase)}
                              onSend={(p) => void handleSend(p)}
                            />
                          ))}
                        </Fragment>
                      );
                    })
                  : sortedGroupRows(groupFilter, rows).map((phrase) => (
                      <PhraseRow
                        key={phrase}
                        phrase={phrase}
                        busy={busy}
                        label={phraseLabel(phrase)}
                        contextualConfirmation={isContextualConfirmation(phrase)}
                        onSend={(p) => void handleSend(p)}
                      />
                    ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Stack>
      </DialogContent>
    </Dialog>
  );
}
