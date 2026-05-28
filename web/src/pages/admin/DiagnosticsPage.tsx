import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Container,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import { useTranslation } from "react-i18next";

import { ApiError } from "../../api/client";
import {
  fetchDiagnosticContent,
  useDiagnosticSources,
  type DiagnosticEntry,
} from "../../api/diagnostics";

function formatBytes(n: number): string {
  if (n < 1024) {
    return `${n} B`;
  }
  if (n < 1024 * 1024) {
    return `${(n / 1024).toFixed(1)} KiB`;
  }
  return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
}

export default function DiagnosticsPage() {
  const { t } = useTranslation(["diagnostics", "common", "errors"]);
  const sources = useDiagnosticSources();

  const [groupId, setGroupId] = useState("");
  const [fileId, setFileId] = useState("");
  const [tailLines, setTailLines] = useState(500);
  const [content, setContent] = useState("");
  const [meta, setMeta] = useState<{
    fileName: string;
    size: number;
    truncated: boolean;
  } | null>(null);
  const [loadingContent, setLoadingContent] = useState(false);
  const [contentError, setContentError] = useState<string | null>(null);

  const groups = sources.data?.groups ?? [];

  const entries: DiagnosticEntry[] = useMemo(() => {
    const g = groups.find((x) => x.id === groupId);
    return g?.entries ?? [];
  }, [groups, groupId]);

  useEffect(() => {
    if (groups.length === 0) {
      return;
    }
    if (!groupId || !groups.some((g) => g.id === groupId)) {
      setGroupId(groups[0].id);
    }
  }, [groups, groupId]);

  useEffect(() => {
    if (entries.length === 0) {
      setFileId("");
      return;
    }
    if (!fileId || !entries.some((e) => e.id === fileId)) {
      setFileId(entries[0].id);
    }
  }, [entries, fileId]);

  const loadContent = useCallback(async () => {
    if (!fileId) {
      return;
    }
    setLoadingContent(true);
    setContentError(null);
    try {
      const res = await fetchDiagnosticContent(fileId, tailLines);
      setContent(res.content);
      setMeta({
        fileName: res.fileName,
        size: res.size,
        truncated: res.truncated,
      });
    } catch (e) {
      setContent("");
      setMeta(null);
      if (e instanceof ApiError) {
        const localised = t(`errors:${e.code}` as const, { defaultValue: "" });
        setContentError(
          localised || t("errors:unknown", { code: e.code }),
        );
      } else {
        setContentError(t("errors:network"));
      }
    } finally {
      setLoadingContent(false);
    }
  }, [fileId, tailLines, t]);

  useEffect(() => {
    void loadContent();
  }, [loadContent]);

  const groupLabel = (id: string, backendLabel: string) =>
    t(`sources.${id}.label`, { defaultValue: backendLabel });

  const entryLabel = (id: string, backendLabel: string) =>
    t(`entries.${id}`, { defaultValue: backendLabel });

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Typography variant="h5" component="h1" gutterBottom>
        {t("title")}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {t("subtitle")}
      </Typography>

      {sources.isLoading && (
        <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
          <CircularProgress />
        </Box>
      )}

      {sources.isError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {t("common:networkError")}
        </Alert>
      )}

      {sources.isSuccess && (
        <Stack spacing={2}>
          <Stack direction={{ xs: "column", sm: "row" }} spacing={2}>
            <FormControl fullWidth size="small">
              <InputLabel id="diag-group-label">{t("groupLabel")}</InputLabel>
              <Select
                labelId="diag-group-label"
                label={t("groupLabel")}
                value={groupId}
                onChange={(e) => setGroupId(e.target.value)}
              >
                {groups.map((g) => (
                  <MenuItem key={g.id} value={g.id}>
                    {groupLabel(g.id, g.label)}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <FormControl fullWidth size="small" disabled={entries.length === 0}>
              <InputLabel id="diag-file-label">{t("fileLabel")}</InputLabel>
              <Select
                labelId="diag-file-label"
                label={t("fileLabel")}
                value={fileId}
                onChange={(e) => setFileId(e.target.value)}
              >
                {entries.length === 0 ? (
                  <MenuItem value="" disabled>
                    {t("noFiles")}
                  </MenuItem>
                ) : (
                  entries.map((e) => (
                    <MenuItem key={e.id} value={e.id}>
                      {entryLabel(e.id, e.label)}
                    </MenuItem>
                  ))
                )}
              </Select>
            </FormControl>

            <TextField
              size="small"
              type="number"
              label={t("tailLines")}
              value={tailLines}
              onChange={(e) =>
                setTailLines(Math.max(1, Number(e.target.value) || 500))
              }
              inputProps={{ min: 1, max: 10000 }}
              sx={{ minWidth: 140 }}
            />

            <Button
              variant="outlined"
              startIcon={
                loadingContent ? (
                  <CircularProgress size={18} />
                ) : (
                  <RefreshIcon />
                )
              }
              onClick={() => void loadContent()}
              disabled={!fileId || loadingContent}
              sx={{ alignSelf: { sm: "center" }, flexShrink: 0 }}
            >
              {t("refresh")}
            </Button>
          </Stack>

          {contentError && (
            <Alert severity="warning">{contentError}</Alert>
          )}

          {meta && (
            <Typography variant="caption" color="text.secondary">
              {meta.fileName} · {formatBytes(meta.size)}
              {meta.truncated ? ` · ${t("truncated")}` : ""}
            </Typography>
          )}

          <Paper
            variant="outlined"
            sx={{
              p: 2,
              bgcolor: "grey.900",
              color: "grey.100",
              overflow: "auto",
              maxHeight: "70vh",
            }}
          >
            <Box
              component="pre"
              sx={{
                m: 0,
                fontFamily: "monospace",
                fontSize: "0.8rem",
                whiteSpace: "pre-wrap",
                wordBreak: "break-word",
              }}
            >
              {loadingContent && !content
                ? t("common:loading")
                : content || t("empty")}
            </Box>
          </Paper>
        </Stack>
      )}
    </Container>
  );
}
