import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Alert,
  Badge,
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
  IconButton,
  Paper,
  Snackbar,
  Stack,
  Tab,
  Tabs,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import LoginIcon from "@mui/icons-material/Login";
import LogoutIcon from "@mui/icons-material/Logout";
import SettingsIcon from "@mui/icons-material/Settings";
import { useTranslation } from "react-i18next";
import {
  useBlocker,
  useNavigate,
  useParams,
  type Blocker,
} from "react-router-dom";

import { useMe } from "../api/auth";
import { ApiError } from "../api/client";
import {
  useInterlocking,
  useJoinInterlocking,
  useLeaveInterlocking,
  type InterlockingOccupant,
} from "../api/interlockings";
import { useLayoutVehicles } from "../api/vehicles";
import TakeoverThrottleOverlay, {
  TakeoverWaitingDialog,
  useTakeoverSignalmanSession,
} from "../components/interlocking/TakeoverThrottleOverlay";
import InterlockingChatPanel from "../components/interlocking/InterlockingChatPanel";
import InterlockingRosterPanel from "../components/interlocking/InterlockingRosterPanel";
import InterlockingTrainAnnouncementsPanel from "../components/interlocking/InterlockingTrainAnnouncementsPanel";
import CommandStationPicker from "../components/throttle/CommandStationPicker";
import RadioStopButton from "../components/throttle/RadioStopButton";
import ThrottleSetupDialog from "../components/throttle/ThrottleSetupDialog";
import AutoDismissAlert from "../components/AutoDismissAlert";
import {
  DccBusProvider,
  useDccBusOptional,
} from "../context/DccBusContext";
import {
  useSocket,
  type CommandStationChangedPayload,
} from "../context/SocketContext";
import { useThrottleCommandStationSelection } from "../hooks/useThrottleCommandStationSelection";
import { useInterlockingRadioInbound } from "../hooks/useInterlockingRadioInbound";

// InterlockingPage renders §6.3d: header with name/location/occupant,
// signalman-only join/leave actions, displacement confirmation,
// navigation guard while occupying, staffed work area (radio stop bar,
// chat + roster + train-announcements panels) and command-station picker for in-motion chips.
export default function InterlockingPage() {
  const params = useParams<{ id: string }>();
  const idNum = Number(params.id);
  const id = Number.isFinite(idNum) && idNum > 0 ? idNum : null;
  const navigate = useNavigate();
  const { t } = useTranslation(["interlocking", "errors", "common"]);

  const me = useMe().data ?? null;
  const layoutId = me?.layoutId ?? null;
  const detail = useInterlocking(id);
  const join = useJoinInterlocking();
  const leave = useLeaveInterlocking();
  const { subscribe } = useSocket();

  const isOccupying = useMemo(() => {
    if (!me || !detail.data?.occupant) return false;
    return detail.data.occupant.userId === me.id;
  }, [me, detail.data?.occupant]);

  const [displacedToast, setDisplacedToast] = useState(false);
  useEffect(() => {
    if (id == null) return;
    return subscribe("interlocking.occupantChanged", (payload) => {
      const data = payload as {
        interlockingId?: number;
        reason?: string;
        occupant?: InterlockingOccupant;
      };
      if (data.interlockingId !== id) return;
      if (
        data.reason === "displaced" &&
        me &&
        !data.occupant?.userId &&
        isOccupying
      ) {
        setDisplacedToast(true);
      }
      if (
        data.reason === "displaced" &&
        me &&
        data.occupant &&
        data.occupant.userId !== me.id &&
        isOccupying
      ) {
        setDisplacedToast(true);
      }
    });
  }, [id, me, isOccupying, subscribe]);

  const [confirmDisplace, setConfirmDisplace] = useState<
    InterlockingOccupant | null
  >(null);
  const [joinError, setJoinError] = useState<string | null>(null);

  const attemptJoin = useCallback(
    async (force: boolean) => {
      if (id == null) return;
      setJoinError(null);
      try {
        await join.mutateAsync({ id, force });
        setConfirmDisplace(null);
      } catch (err) {
        if (err instanceof ApiError && err.code === "interlocking_occupied") {
          const incumbent = detail.data?.occupant ?? null;
          if (incumbent && !force) {
            setConfirmDisplace(incumbent);
            return;
          }
        }
        setJoinError(translateError(err));
      }
    },
    [id, join, detail.data?.occupant],
  );

  const handleLeave = useCallback(async () => {
    if (id == null) return;
    setJoinError(null);
    try {
      await leave.mutateAsync(id);
    } catch (err) {
      setJoinError(translateError(err));
    }
  }, [id, leave]);

  const translateError = useCallback(
    (err: unknown): string => {
      if (err instanceof ApiError) {
        const localised = t(`errors:${err.code}` as const, { defaultValue: "" });
        if (localised) return localised;
        return t("errors:unknown", { code: err.code });
      }
      return t("errors:network");
    },
    [t],
  );

  const blocker = useBlocker(isOccupying);

  if (id == null) {
    return (
      <Container maxWidth="md" sx={{ py: 4 }}>
        <Alert severity="error">{t("errors:invalid_id")}</Alert>
      </Container>
    );
  }

  if (detail.isLoading) {
    return (
      <Stack alignItems="center" py={6}>
        <CircularProgress aria-label={t("common:loading")} />
      </Stack>
    );
  }

  if (detail.error) {
    const code =
      detail.error instanceof ApiError ? detail.error.code : "network";
    return (
      <Container maxWidth="md" sx={{ py: 4 }}>
        <Stack spacing={2}>
          <Button
            startIcon={<ArrowBackIcon />}
            onClick={() => navigate("/")}
            sx={{ alignSelf: "flex-start" }}
          >
            {t("interlocking:view.backToDashboard")}
          </Button>
          <Alert severity="error">
            {t(`errors:${code}` as const, {
              defaultValue: t("errors:unknown", { code }),
            })}
          </Alert>
        </Stack>
      </Container>
    );
  }

  const row = detail.data;
  if (!row) return null;

  const occupant = row.occupant;
  const canJoin = !!me?.isSignalman;
  const busy = join.isPending || leave.isPending;

  return (
    <Container maxWidth="xl" sx={{ py: { xs: 3, sm: 4 } }}>
      <Stack spacing={3}>
        <Box>
          <Button
            startIcon={<ArrowBackIcon />}
            onClick={() => navigate("/")}
            sx={{ mb: 1 }}
          >
            {t("interlocking:view.backToDashboard")}
          </Button>
        </Box>

        <Paper variant="outlined" sx={{ p: { xs: 2, sm: 3 } }}>
          <Stack spacing={2}>
            <Stack
              direction={{ xs: "column", sm: "row" }}
              alignItems={{ xs: "flex-start", sm: "center" }}
              justifyContent="space-between"
              spacing={2}
            >
              <Box>
                <Typography variant="overline" color="text.secondary">
                  {t("interlocking:view.overline")}
                </Typography>
                <Typography variant="h4" component="h1">
                  {row.name}
                </Typography>
                {row.location && (
                  <Typography variant="body2" color="text.secondary" mt={0.5}>
                    {row.location}
                  </Typography>
                )}
              </Box>
              <OccupantBadge occupant={occupant} isMe={isOccupying} />
            </Stack>

            {joinError && (
              <Alert severity="error" onClose={() => setJoinError(null)}>
                {joinError}
              </Alert>
            )}

            {isOccupying || canJoin ? (
              <Stack direction="row" spacing={1} alignItems="center">
                {isOccupying ? (
                  <>
                    <InterlockingSetupButton layoutId={layoutId} />
                    <Button
                      variant="contained"
                      color="warning"
                      startIcon={<LogoutIcon />}
                      onClick={handleLeave}
                      disabled={busy}
                    >
                      {t("interlocking:view.actions.leave")}
                    </Button>
                  </>
                ) : (
                  <Button
                    variant="contained"
                    color="primary"
                    startIcon={<LoginIcon />}
                    onClick={() => attemptJoin(false)}
                    disabled={busy}
                  >
                    {t("interlocking:view.actions.occupy")}
                  </Button>
                )}
              </Stack>
            ) : (
              <Alert severity="info" icon={false}>
                {t("interlocking:view.signalmanOnlyHint")}
              </Alert>
            )}
          </Stack>
        </Paper>

        {isOccupying && layoutId != null && layoutId > 0 && id != null && (
          <InterlockingStaffedWorkArea
            layoutId={layoutId}
            interlockingId={id}
            interlockingName={row.name}
          />
        )}
      </Stack>

      <DisplaceDialog
        incumbent={confirmDisplace}
        busy={join.isPending}
        onConfirm={() => void attemptJoin(true)}
        onCancel={() => setConfirmDisplace(null)}
      />

      <NavigationGuardDialog
        blocker={blocker}
        busy={leave.isPending}
        onConfirm={async () => {
          try {
            await leave.mutateAsync(id);
            blocker.proceed?.();
          } catch (err) {
            setJoinError(translateError(err));
            blocker.reset?.();
          }
        }}
      />

      <Snackbar
        open={displacedToast}
        autoHideDuration={6000}
        onClose={() => setDisplacedToast(false)}
        anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      >
        <Alert
          severity="warning"
          onClose={() => setDisplacedToast(false)}
          sx={{ width: "100%" }}
        >
          {t("interlocking:view.displacedToast")}
        </Alert>
      </Snackbar>
    </Container>
  );
}

function InterlockingSetupButton({ layoutId }: { layoutId: number | null }) {
  const { t } = useTranslation(["interlocking", "throttle"]);
  const [open, setOpen] = useState(false);
  if (layoutId == null || layoutId <= 0) {
    return null;
  }
  return (
    <>
      <IconButton
        color="primary"
        onClick={() => setOpen(true)}
        aria-label={t("throttle:setup.open")}
      >
        <SettingsIcon />
      </IconButton>
      <InterlockingCommandStationDialog
        layoutId={layoutId}
        open={open}
        onClose={() => setOpen(false)}
      />
    </>
  );
}

function InterlockingCommandStationDialog({
  layoutId,
  open,
  onClose,
}: {
  layoutId: number;
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation(["throttle", "interlocking"]);
  const {
    session,
    setCommandStation,
    connected,
    reconnecting,
    subscribe,
  } = useSocket();
  const stations = session?.availableCommandStations ?? [];
  const sessionCommandStationId =
    session?.currentSession?.commandStationId ?? 0;
  const { selectedCS, selectCommandStation } =
    useThrottleCommandStationSelection(
      layoutId,
      stations,
      sessionCommandStationId,
    );
  const [selecting, setSelecting] = useState(false);
  const [spawnError, setSpawnError] = useState<string | null>(null);
  const spawnGenRef = useRef(0);

  useEffect(() => {
    return subscribe("session.commandStationChanged", (raw) => {
      const p = raw as CommandStationChangedPayload;
      if (p.commandStationId !== selectedCS) {
        return;
      }
      if (p.status === "running" && p.wsUrl) {
        setSpawnError(null);
        setSelecting(false);
      } else if (p.status === "degraded") {
        setSpawnError(p.reason ?? "dcc_bus_unavailable");
        setSelecting(false);
      }
    });
  }, [subscribe, selectedCS]);

  useEffect(() => {
    if (!open || !connected || !session?.sessionId || selectedCS <= 0) {
      return;
    }
    const gen = ++spawnGenRef.current;
    setSelecting(true);
    setSpawnError(null);
    void setCommandStation(selectedCS).then((result) => {
      if (gen !== spawnGenRef.current) {
        return;
      }
      setSelecting(false);
      if (!result.ok) {
        setSpawnError(result.error ?? "dcc_bus_unavailable");
      }
    });
  }, [open, connected, session?.sessionId, selectedCS, setCommandStation]);

  return (
    <ThrottleSetupDialog open={open} onClose={onClose}>
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        <Chip
          color={connected ? "success" : "default"}
          label={t(
            connected
              ? "throttle:controlPlane.online"
              : "throttle:controlPlane.offline",
          )}
        />
      </Stack>
      {reconnecting && (
        <AutoDismissAlert severity="warning" resetKey="control-reconnecting">
          {t("throttle:controlPlane.reconnecting")}
        </AutoDismissAlert>
      )}
      <CommandStationPicker
        stations={stations}
        currentID={selectedCS}
        disabled={selecting}
        allowClear
        onChange={(csID) => {
          setSpawnError(null);
          selectCommandStation(csID);
        }}
      />
      {spawnError && (
        <Alert severity="error">{t(`errors:${spawnError}` as const, { defaultValue: spawnError })}</Alert>
      )}
      {selectedCS === 0 && (
        <AutoDismissAlert severity="info" resetKey={`select-cs-${stations.length}`}>
          {stations.length === 0
            ? t("throttle:noCommandStations")
            : t("interlocking:view.setup.selectCommandStation")}
        </AutoDismissAlert>
      )}
    </ThrottleSetupDialog>
  );
}

function InterlockingStaffedWorkArea({
  layoutId,
  interlockingId,
  interlockingName,
}: {
  layoutId: number;
  interlockingId: number;
  interlockingName: string;
}) {
  const theme = useTheme();
  const narrow = useMediaQuery(theme.breakpoints.down("md"));
  const { t } = useTranslation("interlocking");
  const { session } = useSocket();
  const stations = session?.availableCommandStations ?? [];
  const sessionCommandStationId =
    session?.currentSession?.commandStationId ?? 0;
  const { selectedCS } = useThrottleCommandStationSelection(
    layoutId,
    stations,
    sessionCommandStationId,
  );
  const activeWsUrl =
    stations.find((s) => s.id === selectedCS)?.wsUrl ?? null;
  const [tab, setTab] = useState(0);
  const {
    grant,
    clearGrant,
    pending: takeoverPending,
    clearPending: clearTakeoverPending,
    rejected: takeoverRejected,
    clearRejected: clearTakeoverRejected,
  } = useTakeoverSignalmanSession(layoutId);
  const chatVisible = !narrow || tab === 0;
  const rosterVisible = !narrow || tab === 1;
  const announcementsVisible = !narrow || tab === 2;
  const radioInbound = useInterlockingRadioInbound(interlockingId, chatVisible);

  const panels = (
    <Stack
      direction={narrow ? "column" : "row"}
      spacing={2}
      alignItems="stretch"
      sx={{ minHeight: 320 }}
    >
      {chatVisible && (
        <Box sx={{ flex: narrow ? undefined : 1, minWidth: 0 }}>
          <InterlockingChatPanel
            interlockingId={interlockingId}
            unreadCount={radioInbound.unreadCount}
          />
        </Box>
      )}
      {rosterVisible && (
        <Box sx={{ flex: narrow ? undefined : 1, minWidth: 0, maxWidth: narrow ? undefined : 480 }}>
          <InterlockingRosterPanel layoutId={layoutId} />
        </Box>
      )}
      {announcementsVisible && (
        <Box sx={{ flex: narrow ? undefined : 1, minWidth: 0, maxWidth: narrow ? undefined : 360 }}>
          <InterlockingTrainAnnouncementsPanel interlockingName={interlockingName} />
        </Box>
      )}
    </Stack>
  );

  const body = activeWsUrl ? (
    <DccBusProvider wsUrl={activeWsUrl}>
      <RosterMotionSubscriber layoutId={layoutId} />
      {panels}
    </DccBusProvider>
  ) : (
    panels
  );

  return (
    <Stack spacing={2}>
      {radioInbound.alertNode}
      {radioInbound.overlay}
      {takeoverRejected && (
        <AutoDismissAlert
          severity="warning"
          resetKey="takeover-rejected"
          onClose={clearTakeoverRejected}
        >
          {t("view.takeover.rejectedToast")}
        </AutoDismissAlert>
      )}
      {takeoverPending && !grant && (
        <TakeoverWaitingDialog
          pending={takeoverPending}
          onCancel={clearTakeoverPending}
        />
      )}
      <RadioStopButton layoutId={layoutId} variant="bar" />
      {grant && activeWsUrl && (
        <TakeoverThrottleOverlay
          layoutId={layoutId}
          grant={grant}
          wsUrl={activeWsUrl}
          onClose={clearGrant}
        />
      )}
      {narrow && (
        <Tabs
          value={tab}
          onChange={(_, value: number) => setTab(value)}
          variant="fullWidth"
        >
          <Tab
            label={
              <Badge
                color="error"
                badgeContent={radioInbound.unreadCount}
                invisible={radioInbound.unreadCount === 0}
              >
                {t("view.panels.chat")}
              </Badge>
            }
          />
          <Tab label={t("view.panels.roster")} />
          <Tab label={t("view.panels.announcements")} />
        </Tabs>
      )}
      {body}
    </Stack>
  );
}

function RosterMotionSubscriber({ layoutId }: { layoutId: number }) {
  const dcc = useDccBusOptional();
  const vehicles = useLayoutVehiclesForMotion(layoutId);
  const subscribe = dcc?.subscribe;
  const status = dcc?.status;

  useEffect(() => {
    if (!subscribe || vehicles.length === 0 || status !== "open") {
      return;
    }
    void subscribe(vehicles);
  }, [subscribe, status, vehicles]);

  return null;
}

function useLayoutVehiclesForMotion(layoutId: number): number[] {
  const roster = useLayoutVehicles(layoutId).data ?? [];
  return useMemo(() => {
    const addrs: number[] = [];
    for (const v of roster) {
      if (v.dccAddress != null) {
        addrs.push(v.dccAddress);
      }
    }
    return addrs;
  }, [roster]);
}

function OccupantBadge({
  occupant,
  isMe,
}: {
  occupant: InterlockingOccupant | undefined;
  isMe: boolean;
}) {
  const { t } = useTranslation("interlocking");
  if (!occupant) {
    return (
      <Chip
        color="default"
        variant="outlined"
        label={t("view.vacant")}
        size="medium"
      />
    );
  }
  return (
    <Chip
      color={isMe ? "success" : "primary"}
      label={
        isMe
          ? t("view.occupantSelf", { login: occupant.login })
          : t("view.occupant", { login: occupant.login })
      }
      size="medium"
    />
  );
}

function DisplaceDialog({
  incumbent,
  busy,
  onConfirm,
  onCancel,
}: {
  incumbent: InterlockingOccupant | null;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation("interlocking");
  return (
    <Dialog open={incumbent !== null} onClose={onCancel}>
      <DialogTitle>{t("view.displace.title")}</DialogTitle>
      <DialogContent>
        <DialogContentText>
          {incumbent
            ? t("view.displace.message", { login: incumbent.login })
            : null}
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} disabled={busy}>
          {t("view.displace.cancel")}
        </Button>
        <Button onClick={onConfirm} color="warning" disabled={busy} variant="contained">
          {t("view.displace.confirm")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function NavigationGuardDialog({
  blocker,
  busy,
  onConfirm,
}: {
  blocker: Blocker;
  busy: boolean;
  onConfirm: () => Promise<void>;
}) {
  const { t } = useTranslation("interlocking");
  const open = blocker.state === "blocked";

  return (
    <Dialog open={open} onClose={() => blocker.reset?.()}>
      <DialogTitle>
        {t("view.leaveGuard.title")}
        <IconButton
          aria-label={t("view.leaveGuard.cancel")}
          onClick={() => blocker.reset?.()}
          sx={{ position: "absolute", right: 8, top: 8 }}
          disabled={busy}
        >
          <ArrowBackIcon />
        </IconButton>
      </DialogTitle>
      <DialogContent>
        <DialogContentText>
          {t("view.leaveGuard.message")}
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => blocker.reset?.()} disabled={busy}>
          {t("view.leaveGuard.cancel")}
        </Button>
        <Button
          onClick={() => void onConfirm()}
          color="warning"
          variant="contained"
          disabled={busy}
        >
          {t("view.leaveGuard.confirm")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
