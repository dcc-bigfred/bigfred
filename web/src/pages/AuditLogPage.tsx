import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { auditLogQueryKey, useAuditLog, type AuditEntry } from "../api/auditLog";

function formatDate(ms: number): string {
  return new Date(ms).toLocaleString();
}

function AuditMessage({ entry }: { entry: AuditEntry }) {
  const { t, i18n } = useTranslation("audit");

  const vars: Record<string, string> = {
    actorLogin: entry.actorLogin,
    ...entry.vars,
  };

  const fullKey = `audit:events.${entry.msg}`;
  if (!i18n.exists(fullKey)) {
    return <>{t("unknownEvent", { msg: entry.msg })}</>;
  }
  // Dynamic key lookup — cast required because the key is not a
  // compile-time constant and i18next's strict types cannot verify it.
  const tDynamic = t as unknown as (k: string, opts: Record<string, unknown>) => string;
  return <>{tDynamic(`events.${entry.msg}`, vars)}</>;
}

export default function AuditLogPage() {
  const { t } = useTranslation(["audit", "common"]);
  const queryClient = useQueryClient();
  const { data, isFetching, error, refetch } = useAuditLog(200);

  const entries = data?.entries ?? [];

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          mb: 2,
        }}
      >
        <Box>
          <Typography variant="h5" component="h1" gutterBottom>
            {t("audit:title")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("audit:subtitle")}
          </Typography>
        </Box>
        <Button
          startIcon={
            isFetching ? (
              <CircularProgress size={16} color="inherit" />
            ) : (
              <RefreshIcon />
            )
          }
          variant="outlined"
          onClick={() => {
            void queryClient.invalidateQueries({ queryKey: auditLogQueryKey });
            void refetch();
          }}
          disabled={isFetching}
        >
          {t("audit:refresh")}
        </Button>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {t("audit:error")}
        </Alert>
      )}

      {!isFetching && !error && entries.length === 0 && (
        <Alert severity="info">{t("audit:empty")}</Alert>
      )}

      {entries.length > 0 && (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell sx={{ width: 180 }}>{t("audit:colTime")}</TableCell>
                <TableCell>{t("audit:colEvent")}</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {entries.map((entry) => (
                <TableRow key={entry.streamId} hover>
                  <TableCell sx={{ whiteSpace: "nowrap", color: "text.secondary" }}>
                    {formatDate(entry.occurredAt)}
                  </TableCell>
                  <TableCell>
                    <AuditMessage entry={entry} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Container>
  );
}
