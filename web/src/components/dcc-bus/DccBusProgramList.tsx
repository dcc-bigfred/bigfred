import { Alert, CircularProgress, Stack } from "@mui/material";
import { useTranslation } from "react-i18next";

import type {
  DccBusProgramStatus,
  DccBusSupervisordAction,
} from "../../api/command_stations";
import DccBusProgramRow from "./DccBusProgramRow";

export default function DccBusProgramList({
  programs,
  pendingLayoutId,
  errorByLayoutId,
  loading,
  listError,
  onAction,
  onViewLogs,
}: {
  programs: DccBusProgramStatus[];
  pendingLayoutId: number | null;
  errorByLayoutId?: Record<number, string | null>;
  loading?: boolean;
  listError?: string | null;
  onAction: (layoutId: number, action: DccBusSupervisordAction) => void;
  onViewLogs: (layoutId: number) => void;
}) {
  const { t } = useTranslation(["commandStation"]);

  if (loading) {
    return <CircularProgress size={24} />;
  }
  if (listError) {
    return <Alert severity="error">{listError}</Alert>;
  }
  if (programs.length === 0) {
    return (
      <Alert severity="info">
        {t("commandStation:admin.supervisord.noPrograms")}
      </Alert>
    );
  }

  return (
    <Stack spacing={2}>
      {programs.map((program) => (
        <DccBusProgramRow
          key={program.layoutId}
          program={program}
          pendingLayoutId={pendingLayoutId}
          error={errorByLayoutId?.[program.layoutId] ?? null}
          onAction={(next) => onAction(program.layoutId, next)}
          onViewLogs={() => onViewLogs(program.layoutId)}
        />
      ))}
    </Stack>
  );
}
