import {
  Box,
  Button,
  IconButton,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import DeleteIcon from "@mui/icons-material/Delete";
import { useTranslation } from "react-i18next";

import type { DCCAddressRange } from "../api/vehicles";

export const DCC_ADDRESS_MIN = 1;
export const DCC_ADDRESS_MAX = 9999;

export interface DccPoolRangeInput {
  from: string;
  to: string;
}

export function emptyDccPoolRange(): DccPoolRangeInput {
  return { from: "", to: "" };
}

export function dccPoolFromApi(rows: DCCAddressRange[]): DccPoolRangeInput[] {
  if (rows.length === 0) return [emptyDccPoolRange()];
  return rows.map((r) => ({ from: String(r.from), to: String(r.to) }));
}

export function formatDccPoolSummary(rows: DCCAddressRange[]): string {
  if (rows.length === 0) return "—";
  return rows
    .map((r) => (r.from === r.to ? String(r.from) : `${r.from}–${r.to}`))
    .join(", ");
}

function parseAddress(raw: string): number | null {
  if (!/^\d+$/.test(raw)) return null;
  const n = Number(raw);
  if (!Number.isInteger(n) || n < DCC_ADDRESS_MIN || n > DCC_ADDRESS_MAX) {
    return null;
  }
  return n;
}

export function parseDccPoolRanges(
  rows: DccPoolRangeInput[],
): { from: number; to: number }[] | null {
  if (rows.length === 0) return null;
  const out: { from: number; to: number }[] = [];
  for (const row of rows) {
    const from = parseAddress(row.from.trim());
    const to = parseAddress(row.to.trim());
    if (from == null || to == null || from > to) return null;
    out.push({ from, to });
  }
  return out;
}

export function isDccPoolInputValid(rows: DccPoolRangeInput[]): boolean {
  return parseDccPoolRanges(rows) != null;
}

interface Props {
  value: DccPoolRangeInput[];
  onChange: (rows: DccPoolRangeInput[]) => void;
  disabled?: boolean;
}

export default function UserDccPoolFields({ value, onChange, disabled }: Props) {
  const { t } = useTranslation("user");

  const updateRow = (index: number, patch: Partial<DccPoolRangeInput>) => {
    onChange(value.map((row, i) => (i === index ? { ...row, ...patch } : row)));
  };

  const removeRow = (index: number) => {
    if (value.length <= 1) return;
    onChange(value.filter((_, i) => i !== index));
  };

  const addRow = () => {
    onChange([...value, emptyDccPoolRange()]);
  };

  return (
    <Stack spacing={1.5}>
      <Box>
        <Typography variant="subtitle2">
          {t("admin.dialogs.fields.dccPool")}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {t("admin.dialogs.fields.dccPoolHelp", {
            min: DCC_ADDRESS_MIN,
            max: DCC_ADDRESS_MAX,
          })}
        </Typography>
      </Box>
      {value.map((row, index) => (
        <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
          <TextField
            label={t("admin.dialogs.fields.dccFrom")}
            value={row.from}
            onChange={(e) => updateRow(index, { from: e.target.value })}
            disabled={disabled}
            required
            size="small"
            sx={{ flex: 1 }}
            inputProps={{
              inputMode: "numeric",
              pattern: "[0-9]*",
              min: DCC_ADDRESS_MIN,
              max: DCC_ADDRESS_MAX,
            }}
          />
          <TextField
            label={t("admin.dialogs.fields.dccTo")}
            value={row.to}
            onChange={(e) => updateRow(index, { to: e.target.value })}
            disabled={disabled}
            required
            size="small"
            sx={{ flex: 1 }}
            inputProps={{
              inputMode: "numeric",
              pattern: "[0-9]*",
              min: DCC_ADDRESS_MIN,
              max: DCC_ADDRESS_MAX,
            }}
          />
          <IconButton
            size="small"
            onClick={() => removeRow(index)}
            disabled={disabled || value.length <= 1}
            aria-label={t("admin.dialogs.fields.dccRemoveRange")}
            sx={{ mt: 0.5 }}
          >
            <DeleteIcon fontSize="small" />
          </IconButton>
        </Stack>
      ))}
      <Button
        size="small"
        startIcon={<AddIcon />}
        onClick={addRow}
        disabled={disabled}
        sx={{ alignSelf: "flex-start" }}
      >
        {t("admin.dialogs.fields.dccAddRange")}
      </Button>
    </Stack>
  );
}
