import {
  Alert,
  Box,
  CircularProgress,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useVersion } from "../api/version";

function VersionField({
  label,
  children,
  mono = false,
}: {
  label: string;
  children: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <TableRow>
      <TableCell
        component="th"
        scope="row"
        sx={{ width: { sm: "40%" }, fontWeight: 500 }}
      >
        {label}
      </TableCell>
      <TableCell sx={mono ? { fontFamily: "monospace" } : undefined}>
        {children}
      </TableCell>
    </TableRow>
  );
}

/** Shared BigFred version table used by VersionPage and SystemPage. */
export default function VersionCard() {
  const { t } = useTranslation(["version"]);
  const q = useVersion();

  if (q.error) {
    return <Alert severity="error">{t("version:states.error")}</Alert>;
  }
  if (q.isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
        <CircularProgress />
      </Box>
    );
  }
  if (!q.data) return null;

  return (
    <Paper variant="outlined">
      <TableContainer>
        <Table size="small">
          <TableBody>
            <VersionField label={t("version:fields.version")}>
              {q.data.version || "—"}
            </VersionField>
            <VersionField label={t("version:fields.tagCommit")} mono>
              {q.data.tagCommit || "—"}
            </VersionField>
            <VersionField label={t("version:fields.buildCommit")} mono>
              {q.data.buildCommit || "—"}
            </VersionField>
            <VersionField label={t("version:fields.buildTime")}>
              {q.data.buildTime || "—"}
            </VersionField>
          </TableBody>
        </Table>
      </TableContainer>
    </Paper>
  );
}

export function VersionPageHeader() {
  const { t } = useTranslation(["version"]);
  return (
    <Stack spacing={0.5}>
      <Typography variant="h4" component="h1" gutterBottom>
        {t("version:title")}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {t("version:subtitle")}
      </Typography>
    </Stack>
  );
}
