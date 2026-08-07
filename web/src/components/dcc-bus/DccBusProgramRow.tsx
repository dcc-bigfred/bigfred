import {
  Alert,
  Button,
  Chip,
  Stack,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import type {
  DccBusProgramStatus,
  DccBusSupervisordAction,
} from "../../api/command_stations";

export default function DccBusProgramRow({
  program,
  pendingLayoutId,
  error,
  onAction,
  onViewLogs,
}: {
  program: DccBusProgramStatus;
  pendingLayoutId: number | null;
  error: string | null;
  onAction: (action: DccBusSupervisordAction) => void;
  onViewLogs: () => void;
}) {
  const { t } = useTranslation(["commandStation"]);
  const layoutLabel = program.layoutName || `#${program.layoutId}`;
  const busy = pendingLayoutId === program.layoutId;
  const running = program.running;

  return (
    <Stack
      spacing={1}
      sx={{
        border: 1,
        borderColor: "divider",
        borderRadius: 1,
        p: 1.5,
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        alignItems="center"
        justifyContent="space-between"
        flexWrap="wrap"
        useFlexGap
      >
        <Typography variant="subtitle2">{layoutLabel}</Typography>
        <Chip
          size="small"
          color={running ? "success" : "default"}
          label={
            running
              ? t("commandStation:admin.supervisord.statusOn")
              : t("commandStation:admin.supervisord.statusOff")
          }
        />
      </Stack>
      <Typography variant="caption" color="text.secondary">
        {program.name}
        {program.pid != null && program.pid > 0 ? ` · pid ${program.pid}` : ""}
      </Typography>
      {error && <Alert severity="error">{error}</Alert>}
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        <Button
          variant="contained"
          size="small"
          disabled={busy || running}
          onClick={() => onAction("start")}
        >
          {t("commandStation:admin.supervisord.start")}
        </Button>
        <Button
          variant="outlined"
          size="small"
          disabled={busy || !running}
          onClick={() => onAction("stop")}
        >
          {t("commandStation:admin.supervisord.stop")}
        </Button>
        <Button
          variant="outlined"
          size="small"
          disabled={busy}
          onClick={() => onAction("restart")}
        >
          {t("commandStation:admin.supervisord.restart")}
        </Button>
        <Button variant="text" size="small" onClick={onViewLogs}>
          {t("commandStation:admin.supervisord.viewLogs")}
        </Button>
      </Stack>
    </Stack>
  );
}
