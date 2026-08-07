import { useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  Typography,
} from "@mui/material";
import PauseIcon from "@mui/icons-material/Pause";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import DeleteOutlineIcon from "@mui/icons-material/DeleteOutline";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";

import { useMicroinitLogStream } from "../../api/microinitLogs";
import { useMicroinitServices } from "../../api/system";

export default function DiagnosticsPage() {
  const { t } = useTranslation(["diagnostics", "common", "errors"]);
  const [searchParams] = useSearchParams();
  const services = useMicroinitServices();

  const preferredService = searchParams.get("service") ?? "";
  const [serviceName, setServiceName] = useState("");
  const appliedPref = useRef(false);

  const serviceList = useMemo(
    () => services.data ?? [],
    [services.data],
  );

  useEffect(() => {
    if (serviceList.length === 0) return;
    if (
      !appliedPref.current &&
      preferredService &&
      serviceList.some((s) => s.name === preferredService)
    ) {
      setServiceName(preferredService);
      appliedPref.current = true;
      return;
    }
    // Deep-link may name a service not yet listed — still select it.
    if (!appliedPref.current && preferredService) {
      setServiceName(preferredService);
      appliedPref.current = true;
      return;
    }
    setServiceName((current) =>
      current &&
      (serviceList.some((s) => s.name === current) || current === preferredService)
        ? current
        : serviceList[0]?.name ?? "",
    );
  }, [serviceList, preferredService]);

  const stream = useMicroinitLogStream(
    serviceName || null,
    Boolean(serviceName),
  );

  const preRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (stream.paused) return;
    const el = preRef.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [stream.lines, stream.paused]);

  const stateLabel = (() => {
    switch (stream.state) {
      case "connecting":
        return t("diagnostics:live.connecting");
      case "live":
        return t("diagnostics:live.connected");
      case "error":
        return t("diagnostics:live.error");
      case "closed":
        return t("diagnostics:live.reconnecting");
      default:
        return t("diagnostics:live.idle");
    }
  })();

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Typography variant="h5" component="h1" gutterBottom sx={{ mb: 3 }}>
        {t("title")}
      </Typography>

      {services.isLoading && (
        <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
          <CircularProgress />
        </Box>
      )}

      {services.isError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {t("diagnostics:unavailable")}
        </Alert>
      )}

      {(services.isSuccess || preferredService) && (
        <Stack spacing={2}>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={2}
            alignItems={{ sm: "center" }}
          >
            <FormControl fullWidth size="small">
              <InputLabel id="diag-service-label">
                {t("diagnostics:serviceLabel")}
              </InputLabel>
              <Select
                labelId="diag-service-label"
                label={t("diagnostics:serviceLabel")}
                value={serviceName}
                onChange={(e) => setServiceName(e.target.value)}
              >
                {serviceList.length === 0 && serviceName ? (
                  <MenuItem value={serviceName}>{serviceName}</MenuItem>
                ) : null}
                {serviceList.map((s) => (
                  <MenuItem key={s.name} value={s.name}>
                    {s.name}
                    {s.state ? ` (${s.state})` : ""}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <Chip
              size="small"
              color={
                stream.state === "live"
                  ? "success"
                  : stream.state === "error"
                    ? "error"
                    : "default"
              }
              label={stateLabel}
              sx={{ flexShrink: 0 }}
            />

            <Button
              variant="outlined"
              size="small"
              startIcon={stream.paused ? <PlayArrowIcon /> : <PauseIcon />}
              onClick={() => stream.setPaused(!stream.paused)}
              disabled={!serviceName}
              sx={{ flexShrink: 0 }}
            >
              {stream.paused
                ? t("diagnostics:live.resume")
                : t("diagnostics:live.pause")}
            </Button>

            <Button
              variant="outlined"
              size="small"
              startIcon={<DeleteOutlineIcon />}
              onClick={stream.clear}
              disabled={!serviceName}
              sx={{ flexShrink: 0 }}
            >
              {t("diagnostics:live.clear")}
            </Button>
          </Stack>

          {stream.error && (
            <Alert severity="warning">{stream.error}</Alert>
          )}

          <Paper
            variant="outlined"
            ref={preRef}
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
              {stream.lines.length === 0
                ? stream.state === "connecting"
                  ? t("common:loading")
                  : t("diagnostics:empty")
                : stream.lines.join("\n")}
            </Box>
          </Paper>
        </Stack>
      )}
    </Container>
  );
}
