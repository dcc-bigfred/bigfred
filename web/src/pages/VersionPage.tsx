import {
  Alert,
  Box,
  CircularProgress,
  Container,
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
      <TableCell component="th" scope="row" sx={{ width: { sm: "40%" }, fontWeight: 500 }}>
        {label}
      </TableCell>
      <TableCell sx={mono ? { fontFamily: "monospace" } : undefined}>{children}</TableCell>
    </TableRow>
  );
}

export default function VersionPage() {
  const { t } = useTranslation(["version", "common"]);
  const q = useVersion();

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Box>
          <Typography variant="h4" component="h1" gutterBottom>
            {t("version:title")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("version:subtitle")}
          </Typography>
        </Box>

        {q.error && <Alert severity="error">{t("version:states.error")}</Alert>}

        {q.isLoading ? (
          <Box sx={{ display: "flex", justifyContent: "center", py: 6 }}>
            <CircularProgress />
          </Box>
        ) : q.data ? (
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
        ) : null}
      </Stack>
    </Container>
  );
}
