import { useCallback, useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  CircularProgress,
  Container,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import AnalyticsIcon from "@mui/icons-material/Analytics";
import ArticleIcon from "@mui/icons-material/Article";
import DnsIcon from "@mui/icons-material/Dns";
import PowerSettingsNewIcon from "@mui/icons-material/PowerSettingsNew";
import RefreshIcon from "@mui/icons-material/Refresh";
import RestartAltIcon from "@mui/icons-material/RestartAlt";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useMe } from "../../api/auth";
import { ApiError } from "../../api/client";
import {
  useCommandStationsCatalogue,
  useDccBusSupervisordAction,
  useDccBusSupervisordStatus,
  type DccBusSupervisordAction,
} from "../../api/command_stations";
import { useLayoutSupervisordSync } from "../../api/presence";
import {
  useMicroinitInfo,
  useMicroinitServices,
  useSystemInfo,
  useSystemPorts,
  type SystemShutdownMode,
} from "../../api/system";
import DccBusProgramList from "../../components/dcc-bus/DccBusProgramList";
import SystemPowerDialog from "../../components/SystemPowerDialog";
import VersionCard from "../../components/VersionCard";

function formatUptime(secs: number): string {
  if (!Number.isFinite(secs) || secs < 0) return "—";
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const s = Math.floor(secs % 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function ActionTile({
  icon,
  label,
  disabled,
  disabledReason,
  onClick,
  danger,
}: {
  icon: React.ReactNode;
  label: string;
  disabled?: boolean;
  disabledReason?: string;
  onClick: () => void;
  danger?: boolean;
}) {
  const card = (
    <Card
      variant="outlined"
      sx={{
        height: "100%",
        opacity: disabled ? 0.5 : 1,
        borderColor: danger ? "error.light" : undefined,
      }}
    >
      <CardActionArea
        disabled={disabled}
        onClick={onClick}
        sx={{ height: "100%", minHeight: 140 }}
      >
        <CardContent
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 1.5,
            py: 3,
            textAlign: "center",
          }}
        >
          <Box
            sx={{
              fontSize: 48,
              lineHeight: 1,
              color: danger ? "error.main" : "primary.main",
              display: "flex",
            }}
          >
            {icon}
          </Box>
          <Typography variant="subtitle1" fontWeight={600}>
            {label}
          </Typography>
        </CardContent>
      </CardActionArea>
    </Card>
  );
  if (disabled && disabledReason) {
    return (
      <Tooltip title={disabledReason}>
        <Box sx={{ height: "100%" }}>{card}</Box>
      </Tooltip>
    );
  }
  return card;
}

function CommandStationDaemons({
  csId,
  csName,
}: {
  csId: number;
  csName: string;
}) {
  const { t } = useTranslation(["commandStation", "errors", "system"]);
  const navigate = useNavigate();
  const status = useDccBusSupervisordStatus(csId);
  const action = useDccBusSupervisordAction(csId);
  const [pendingLayoutId, setPendingLayoutId] = useState<number | null>(null);
  const [failedLayoutId, setFailedLayoutId] = useState<number | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const formatError = (err: unknown): string => {
    if (err instanceof ApiError) {
      if (err.status === 503 || err.code === "service_unavailable") {
        return t("commandStation:admin.supervisord.unavailable");
      }
      return (
        err.detail ||
        t(`errors:${err.code}` as const, {
          defaultValue: t("commandStation:admin.supervisord.actionFailed"),
        })
      );
    }
    return t("errors:network");
  };

  const runAction = async (
    layoutId: number,
    next: DccBusSupervisordAction,
  ) => {
    setActionError(null);
    setFailedLayoutId(null);
    setPendingLayoutId(layoutId);
    try {
      await action.mutateAsync({ action: next, layoutId });
    } catch (err) {
      setFailedLayoutId(layoutId);
      setActionError(formatError(err));
    } finally {
      setPendingLayoutId(null);
    }
  };

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Typography variant="subtitle1" fontWeight={600} gutterBottom>
        {csName}
      </Typography>
      <DccBusProgramList
        programs={status.data?.programs ?? []}
        pendingLayoutId={pendingLayoutId}
        errorByLayoutId={
          failedLayoutId != null && actionError
            ? { [failedLayoutId]: actionError }
            : undefined
        }
        loading={status.isLoading}
        listError={status.isError ? formatError(status.error) : null}
        onAction={(layoutId, next) => void runAction(layoutId, next)}
        onViewLogs={(layoutId) =>
          navigate(
            `/admin/logs?service=${encodeURIComponent(`dcc-bus-${layoutId}-${csId}`)}`,
          )
        }
      />
    </Paper>
  );
}

export default function SystemPage() {
  const { t } = useTranslation(["system", "common", "version", "commandStation"]);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const me = useMe().data;
  const layoutId = me?.layoutId ?? null;
  useLayoutSupervisordSync(layoutId);

  const systemInfo = useSystemInfo();
  const ports = useSystemPorts();
  const microInfo = useMicroinitInfo();
  const microServices = useMicroinitServices();
  const catalogue = useCommandStationsCatalogue();

  const [powerOpen, setPowerOpen] = useState(false);
  const [powerMode, setPowerMode] = useState<SystemShutdownMode | undefined>();
  const [refreshingDcc, setRefreshingDcc] = useState(false);

  const canShutdown = systemInfo.data?.canShutdown === true;
  const host = window.location.hostname;

  const openPower = (mode: SystemShutdownMode) => {
    setPowerMode(mode);
    setPowerOpen(true);
  };

  const refreshDccBus = useCallback(async () => {
    if (layoutId == null || layoutId <= 0) return;
    setRefreshingDcc(true);
    try {
      await qc.refetchQueries({
        queryKey: ["layouts", layoutId, "presence"],
      });
      await new Promise((r) => setTimeout(r, 1000));
      await Promise.all([
        qc.invalidateQueries({ queryKey: ["admin", "dcc-bus"] }),
        qc.invalidateQueries({ queryKey: ["admin", "microinit", "services"] }),
      ]);
    } finally {
      setRefreshingDcc(false);
    }
  }, [layoutId, qc]);

  const stations = catalogue.data ?? [];

  const serviceRows = useMemo(
    () => microServices.data ?? [],
    [microServices.data],
  );

  return (
    <Container maxWidth="lg" sx={{ py: 3 }}>
      <Stack spacing={4}>
        <Typography variant="h4" component="h1">
          {t("system:title")}
        </Typography>

        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "1fr 1fr",
              md: "repeat(3, 1fr)",
              lg: "repeat(5, 1fr)",
            },
          }}
        >
          <ActionTile
            icon={<RestartAltIcon sx={{ fontSize: 48 }} />}
            label={t("system:actions.reboot")}
            disabled={!canShutdown}
            disabledReason={t("system:actions.powerUnavailable")}
            onClick={() => openPower("reboot")}
          />
          <ActionTile
            icon={<PowerSettingsNewIcon sx={{ fontSize: 48 }} />}
            label={t("system:actions.poweroff")}
            disabled={!canShutdown}
            disabledReason={t("system:actions.powerUnavailable")}
            danger
            onClick={() => openPower("poweroff")}
          />
          <ActionTile
            icon={<ArticleIcon sx={{ fontSize: 48 }} />}
            label={t("system:actions.logs")}
            onClick={() => navigate("/admin/logs")}
          />
          <ActionTile
            icon={<DnsIcon sx={{ fontSize: 48 }} />}
            label={t("system:actions.osPanel")}
            disabled={!ports.data?.osUI}
            disabledReason={t("system:actions.portClosed")}
            onClick={() => window.open(`http://${host}:8090`, "_blank")}
          />
          <ActionTile
            icon={<AnalyticsIcon sx={{ fontSize: 48 }} />}
            label={t("system:actions.grafana")}
            disabled={!ports.data?.grafana}
            disabledReason={t("system:actions.portClosed")}
            onClick={() => window.open(`http://${host}:3000`, "_blank")}
          />
        </Box>

        <Stack spacing={2}>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={1}
            alignItems={{ sm: "center" }}
            justifyContent="space-between"
          >
            <Typography variant="h5">{t("system:microinit.title")}</Typography>
            <Button
              variant="outlined"
              startIcon={
                refreshingDcc ? (
                  <CircularProgress size={16} />
                ) : (
                  <RefreshIcon />
                )
              }
              disabled={refreshingDcc || layoutId == null}
              onClick={() => void refreshDccBus()}
            >
              {t("system:refreshDccBus")}
            </Button>
          </Stack>

          {microInfo.isError ? (
            <Alert severity="warning">{t("system:microinit.unavailable")}</Alert>
          ) : microInfo.isLoading ? (
            <CircularProgress size={24} />
          ) : microInfo.data ? (
            <Paper variant="outlined" sx={{ p: 2 }}>
              <Stack
                direction="row"
                spacing={1}
                flexWrap="wrap"
                useFlexGap
                sx={{ mb: 1 }}
              >
                <Chip
                  size="small"
                  label={`${t("system:microinit.version")}: ${microInfo.data.version || "—"}`}
                />
                <Chip
                  size="small"
                  label={`${t("system:microinit.mode")}: ${microInfo.data.mode || "—"}`}
                />
                <Chip
                  size="small"
                  label={`${t("system:microinit.uptime")}: ${formatUptime(microInfo.data.uptime_secs)}`}
                />
                <Chip
                  size="small"
                  label={`${t("system:microinit.services")}: ${microInfo.data.services_running}/${microInfo.data.services_total}`}
                />
                <Chip
                  size="small"
                  color={microInfo.data.otel_enabled ? "success" : "default"}
                  label={`otel: ${microInfo.data.otel_enabled ? "on" : "off"}`}
                />
              </Stack>
              <Typography variant="caption" color="text.secondary">
                {microInfo.data.hostname}
                {microInfo.data.socket ? ` · ${microInfo.data.socket}` : ""}
              </Typography>
            </Paper>
          ) : null}

          <TableContainer component={Paper} variant="outlined">
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>{t("system:microinit.colName")}</TableCell>
                  <TableCell>{t("system:microinit.colState")}</TableCell>
                  <TableCell>{t("system:microinit.colPid")}</TableCell>
                  <TableCell>{t("system:microinit.colRestarts")}</TableCell>
                  <TableCell>{t("system:microinit.colEnabled")}</TableCell>
                  <TableCell align="right">
                    {t("system:microinit.colLogs")}
                  </TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {microServices.isLoading ? (
                  <TableRow>
                    <TableCell colSpan={6}>
                      <CircularProgress size={20} />
                    </TableCell>
                  </TableRow>
                ) : serviceRows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6}>
                      {t("system:microinit.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  serviceRows.map((svc) => (
                    <TableRow key={svc.name}>
                      <TableCell>
                        <Typography variant="body2" fontFamily="monospace">
                          {svc.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          color={
                            svc.state === "running" ? "success" : "default"
                          }
                          label={svc.state}
                        />
                      </TableCell>
                      <TableCell>{svc.pid ?? "—"}</TableCell>
                      <TableCell>{svc.restarts}</TableCell>
                      <TableCell>
                        {svc.enabled ? "✓" : "—"}
                      </TableCell>
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          aria-label={t("system:microinit.colLogs")}
                          onClick={() =>
                            navigate(
                              `/admin/logs?service=${encodeURIComponent(svc.name)}`,
                            )
                          }
                        >
                          <ArticleIcon fontSize="small" />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Stack>

        <Stack spacing={2}>
          <Typography variant="h5">{t("system:dccBusSection")}</Typography>
          {catalogue.isLoading ? (
            <CircularProgress size={24} />
          ) : stations.length === 0 ? (
            <Alert severity="info">{t("system:dccBusEmpty")}</Alert>
          ) : (
            stations.map((cs) => (
              <CommandStationDaemons
                key={cs.id}
                csId={cs.id}
                csName={cs.name}
              />
            ))
          )}
        </Stack>

        <Stack spacing={2}>
          <Typography variant="h5">{t("version:title")}</Typography>
          <VersionCard />
        </Stack>
      </Stack>

      <SystemPowerDialog
        open={powerOpen}
        onClose={() => {
          setPowerOpen(false);
          setPowerMode(undefined);
        }}
        presetMode={powerMode}
      />
    </Container>
  );
}
