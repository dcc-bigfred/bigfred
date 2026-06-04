import { Chip, Stack } from "@mui/material";

import type { DccFunction } from "../../api/functions";
import { FunctionIconVisual } from "./functionIconMap";

const MAX_VISIBLE = 3;

export default function FunctionSummaryChips({
  functions,
  maxVisible = MAX_VISIBLE,
}: {
  functions: DccFunction[];
  maxVisible?: number;
}) {
  const sorted = [...functions].sort((a, b) => a.position - b.position);
  const visible = sorted.slice(0, maxVisible);
  const hasMore = sorted.length > maxVisible;

  return (
    <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap alignItems="center">
      {visible.map((f) => (
        <Chip
          key={f.num}
          size="small"
          icon={<FunctionIconVisual icon={f.icon} size={16} />}
          label={f.name}
          variant="outlined"
        />
      ))}
      {hasMore && (
        <Chip size="small" label="…" variant="outlined" aria-label="…" />
      )}
    </Stack>
  );
}
