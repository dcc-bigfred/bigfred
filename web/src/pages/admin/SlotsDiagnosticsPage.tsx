import { useCallback, useEffect, useMemo, useState } from "react";
import { Link as RouterLink, useSearchParams } from "react-router-dom";
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControl,
  InputLabel,
  LinearProgress,
  MenuItem,
  Paper,
  Select,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { useTranslation } from "react-i18next";

import { useCommandStationsCatalogue } from "../../api/command_stations";
import { apiFetch } from "../../api/client";
import { useWsConnection } from "../../hooks/useWsConnection";

interface HolderInfo {
  userId: number;
  session: string;
  source: string;
  lastDriveAt: number;
}

function formatSlotSource(source: string): string {
  return source === "withrottle" ? "WiFred" : source;
}

interface LeaseInfo {
  addr: number;
  kind: string;
  trainId?: string;
  holders: HolderInfo[];
  acquiredAt: number;
  releasePending: boolean;
}

interface SlotsDiagnostic {
  maxPerUser: number;
  maxSlots: number;
  used: number;
  perUser: Record<string, number>;
  leases: LeaseInfo[];
  at: number;
}

function resolveWsUrl(path: string): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}${path}`;
}

function leaseHolders(le: LeaseInfo): HolderInfo[] {
  return Array.isArray(le.holders) ? le.holders : [];
}

function formatRelative(ms: number, now: number): string {
  if (ms <= 0) {
    return "—";
  }
  const sec = Math.max(0, Math.floor((now - ms) / 1000));
  if (sec < 60) {
    return `${sec}s`;
  }
  return `${Math.floor(sec / 60)}m ${sec % 60}s`;
}

export default function SlotsDiagnosticsPage() {
  const { t } = useTranslation(["commandStation", "common"]);
  const [searchParams, setSearchParams] = useSearchParams();
  const catalogue = useCommandStationsCatalogue();
  const stations = catalogue.data ?? [];

  const csParam = searchParams.get("cs");
  const selectedId = csParam ? Number(csParam) : stations[0]?.id ?? 0;

  const selectedStation = useMemo(
    () => stations.find((s) => s.id === selectedId),
    [stations, selectedId],
  );

  const wsPath =
    selectedId > 0
      ? `/api/v1/admin/dcc-bus/${selectedId}/slots/ws`
      : null;
  const wsUrl = wsPath ? resolveWsUrl(wsPath) : null;

  const [snapshot, setSnapshot] = useState<SlotsDiagnostic | null>(null);
  const [connected, setConnected] = useState(false);
  const [now, setNow] = useState(Date.now());
  const [releaseAddr, setReleaseAddr] = useState<number | null>(null);
  const [releasing, setReleasing] = useState(false);

  const confirmRelease = useCallback(async () => {
    if (releaseAddr == null || selectedId <= 0) {
      return;
    }
    setReleasing(true);
    try {
      await apiFetch(`/api/v1/admin/dcc-bus/${selectedId}/slots/release`, {
        method: "POST",
        body: JSON.stringify({ addr: releaseAddr }),
      });
    } finally {
      setReleasing(false);
      setReleaseAddr(null);
    }
  }, [releaseAddr, selectedId]);

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);

  const onMessage = useCallback((data: string) => {
    try {
      const msg = JSON.parse(data) as { type?: string; payload?: SlotsDiagnostic };
      if (msg.type === "slots.snapshot" && msg.payload) {
        setSnapshot(msg.payload);
      }
    } catch {
      // ignore malformed frames
    }
  }, []);

  const { reconnecting } = useWsConnection({
    url: wsUrl,
    pingIntervalMs: 300_000,
    onOpen: () => setConnected(true),
    onClose: () => setConnected(false),
    onDispose: () => {
      setConnected(false);
      setSnapshot(null);
    },
    onMessage,
  });

  const stale = snapshot != null && now - snapshot.at > 3000;
  const idleSecs = selectedStation?.idleTimeoutSecs ?? 60;
  const idleWarnMs = idleSecs * 800;

  const sourceCounts = useMemo(() => {
    const counts = { ws: 0, z21: 0, withrottle: 0 };
    for (const le of snapshot?.leases ?? []) {
      const seen = new Set<string>();
      for (const h of leaseHolders(le)) {
        if (!seen.has(h.source)) {
          seen.add(h.source);
          if (h.source in counts) {
            counts[h.source as keyof typeof counts]++;
          }
        }
      }
    }
    return counts;
  }, [snapshot]);

  const perUserRows = useMemo(() => {
    if (!snapshot) {
      return [];
    }
    return Object.entries(snapshot.perUser)
      .map(([uid, n]) => ({ userId: Number(uid), count: n }))
      .sort((a, b) => b.count - a.count);
  }, [snapshot]);

  const budgetPct =
    snapshot && snapshot.maxSlots > 0
      ? Math.min(100, (snapshot.used / snapshot.maxSlots) * 100)
      : 0;
  const headroom =
    snapshot && snapshot.maxSlots > 0
      ? Math.max(0, snapshot.maxSlots - snapshot.used)
      : null;

  return (
    <Container maxWidth="lg" sx={{ py: { xs: 3, sm: 5 } }}>
      <Stack spacing={3}>
        <Stack direction="row" alignItems="center" spacing={2}>
          <Typography
            component={RouterLink}
            to="/admin/command-stations"
            variant="body2"
            sx={{ display: "flex", alignItems: "center", gap: 0.5, textDecoration: "none" }}
          >
            <ArrowBackIcon fontSize="small" />
            {t("commandStation:admin.slotsDiag.back")}
          </Typography>
        </Stack>

        <Box>
          <Typography variant="h4" component="h1" gutterBottom>
            {t("commandStation:admin.slotsDiag.title")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("commandStation:admin.slotsDiag.subtitle")}
          </Typography>
        </Box>

        <Stack direction={{ xs: "column", sm: "row" }} spacing={2} alignItems="center">
          <FormControl size="small" sx={{ minWidth: 240 }}>
            <InputLabel id="cs-select-label">
              {t("commandStation:admin.slotsDiag.station")}
            </InputLabel>
            <Select
              labelId="cs-select-label"
              label={t("commandStation:admin.slotsDiag.station")}
              value={selectedId || ""}
              onChange={(e) => setSearchParams({ cs: String(e.target.value) })}
            >
              {stations.map((s) => (
                <MenuItem key={s.id} value={s.id}>
                  {s.name} (#{s.id})
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <Chip
            size="small"
            color={connected && !stale ? "success" : stale ? "warning" : "default"}
            label={
              reconnecting
                ? t("commandStation:admin.slotsDiag.reconnecting")
                : connected
                  ? stale
                    ? t("commandStation:admin.slotsDiag.stale")
                    : t("commandStation:admin.slotsDiag.live")
                  : t("commandStation:admin.slotsDiag.disconnected")
            }
          />
        </Stack>

        {catalogue.isLoading && (
          <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
            <CircularProgress />
          </Box>
        )}

        {!catalogue.isLoading && stations.length === 0 && (
          <Alert severity="info">{t("commandStation:admin.empty")}</Alert>
        )}

        {snapshot && (
          <>
            <Paper variant="outlined" sx={{ p: 2 }}>
              <Stack spacing={2}>
                <Typography variant="subtitle1">
                  {t("commandStation:admin.slotsDiag.budgetTitle")}
                </Typography>
                {snapshot.maxSlots > 0 ? (
                  <>
                    <Typography variant="body2">
                      {t("commandStation:admin.slotsDiag.budgetUsed", {
                        used: snapshot.used,
                        max: snapshot.maxSlots,
                        headroom,
                      })}
                    </Typography>
                    <LinearProgress variant="determinate" value={budgetPct} />
                  </>
                ) : (
                  <Typography variant="body2">
                    {t("commandStation:admin.slotsDiag.unlimitedSlots", {
                      used: snapshot.used,
                    })}
                  </Typography>
                )}
                <Stack direction="row" spacing={2} flexWrap="wrap" useFlexGap>
                  <Chip
                    size="small"
                    label={t("commandStation:admin.slotsDiag.maxPerUser", {
                      n: snapshot.maxPerUser,
                    })}
                  />
                  <Chip size="small" label={`ws: ${sourceCounts.ws}`} />
                  <Chip size="small" label={`z21: ${sourceCounts.z21}`} />
                  <Chip size="small" label={`WiFred: ${sourceCounts.withrottle}`} />
                </Stack>
              </Stack>
            </Paper>

            <Paper variant="outlined">
              <Box sx={{ p: 2 }}>
                <Typography variant="subtitle1" gutterBottom>
                  {t("commandStation:admin.slotsDiag.perUserTitle")}
                </Typography>
              </Box>
              <TableContainer>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>{t("commandStation:admin.slotsDiag.userId")}</TableCell>
                      <TableCell>{t("commandStation:admin.slotsDiag.driven")}</TableCell>
                      <TableCell>{t("commandStation:admin.slotsDiag.capBar")}</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {perUserRows.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={3}>
                          <Typography variant="body2" color="text.secondary">
                            {t("commandStation:admin.slotsDiag.noDrivers")}
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      perUserRows.map((row) => (
                        <TableRow key={row.userId}>
                          <TableCell>{row.userId}</TableCell>
                          <TableCell>
                            {row.count} / {snapshot.maxPerUser}
                          </TableCell>
                          <TableCell sx={{ minWidth: 160 }}>
                            <LinearProgress
                              variant="determinate"
                              value={Math.min(
                                100,
                                (row.count / snapshot.maxPerUser) * 100,
                              )}
                              color={
                                row.count >= snapshot.maxPerUser ? "error" : "primary"
                              }
                            />
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Paper>

            <Paper variant="outlined">
              <Box sx={{ p: 2 }}>
                <Typography variant="subtitle1" gutterBottom>
                  {t("commandStation:admin.slotsDiag.tableTitle")}
                </Typography>
              </Box>
              <TableContainer>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>{t("commandStation:admin.slotsDiag.addr")}</TableCell>
                      <TableCell>{t("commandStation:admin.slotsDiag.kind")}</TableCell>
                      <TableCell>{t("commandStation:admin.slotsDiag.holders")}</TableCell>
                      <TableCell>{t("commandStation:admin.slotsDiag.acquired")}</TableCell>
                      <TableCell>{t("commandStation:admin.slotsDiag.flags")}</TableCell>
                      <TableCell align="right">
                        {t("commandStation:admin.slotsDiag.actions")}
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {(snapshot.leases ?? []).length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6}>
                          <Typography variant="body2" color="text.secondary">
                            {t("commandStation:admin.slotsDiag.noLeases")}
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      (snapshot.leases ?? []).map((le) => {
                        const holders = leaseHolders(le);
                        const remoteIdle = holders.every(
                          (h) => h.source !== "ws" && h.lastDriveAt > 0,
                        );
                        const youngest = holders.reduce(
                          (y, h) => (h.lastDriveAt > y ? h.lastDriveAt : y),
                          0,
                        );
                        const imminent =
                          remoteIdle &&
                          youngest > 0 &&
                          now - youngest >= idleWarnMs;
                        return (
                          <TableRow
                            key={le.addr}
                            sx={imminent ? { bgcolor: "warning.light" } : undefined}
                          >
                            <TableCell>{le.addr}</TableCell>
                            <TableCell>
                              {le.kind === "train"
                                ? t("commandStation:admin.slotsDiag.trainKind", {
                                    id: le.trainId,
                                  })
                                : t("commandStation:admin.slotsDiag.singleKind")}
                            </TableCell>
                            <TableCell>
                              {holders.map((h) => (
                                <Typography key={`${h.userId}-${h.session}`} variant="body2">
                                  {h.userId} · {formatSlotSource(h.source)} · {h.session} ·{" "}
                                  {h.source === "ws"
                                    ? "—"
                                    : formatRelative(h.lastDriveAt, now)}
                                </Typography>
                              ))}
                            </TableCell>
                            <TableCell>{formatRelative(le.acquiredAt, now)}</TableCell>
                            <TableCell>
                              {le.releasePending && (
                                <Chip
                                  size="small"
                                  color="warning"
                                  label={t("commandStation:admin.slotsDiag.releasePending")}
                                />
                              )}
                            </TableCell>
                            <TableCell align="right">
                              <Button
                                size="small"
                                color="error"
                                variant="outlined"
                                onClick={() => setReleaseAddr(le.addr)}
                              >
                                {t("commandStation:admin.slotsDiag.release")}
                              </Button>
                            </TableCell>
                          </TableRow>
                        );
                      })
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Paper>
          </>
        )}
      </Stack>

      <Dialog open={releaseAddr != null} onClose={() => setReleaseAddr(null)}>
        <DialogTitle>{t("commandStation:admin.slotsDiag.releaseTitle")}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t("commandStation:admin.slotsDiag.releaseConfirm", {
              addr: releaseAddr ?? 0,
            })}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setReleaseAddr(null)} disabled={releasing}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            color="error"
            onClick={() => void confirmRelease()}
            disabled={releasing}
          >
            {t("commandStation:admin.slotsDiag.release")}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}
