import { useCallback, useEffect, useMemo, useState } from "react";
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
  IconButton,
  Paper,
  Snackbar,
  Stack,
  Typography,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import LoginIcon from "@mui/icons-material/Login";
import LogoutIcon from "@mui/icons-material/Logout";
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
import { useSocket } from "../context/SocketContext";

// InterlockingPage renders §6.3d: header with name/location/occupant,
// signalman-only join/leave actions, displacement confirmation,
// navigation guard while occupying and a displaced-user toast.
//
// Radio panel and takeover are intentionally out of scope here; they
// land in a later milestone together with the WS protocol pieces.
export default function InterlockingPage() {
  const params = useParams<{ id: string }>();
  const idNum = Number(params.id);
  const id = Number.isFinite(idNum) && idNum > 0 ? idNum : null;
  const navigate = useNavigate();
  const { t } = useTranslation(["interlocking", "errors", "common"]);

  const me = useMe().data ?? null;
  const detail = useInterlocking(id);
  const join = useJoinInterlocking();
  const leave = useLeaveInterlocking();
  const { subscribe } = useSocket();

  // Local "am I occupying" tracker. Derived from the REST/WS occupant
  // identity rather than the mutation result so it survives reloads
  // and stays correct when another tab of the same user joins/leaves.
  const isOccupying = useMemo(() => {
    if (!me || !detail.data?.occupant) return false;
    return detail.data.occupant.userId === me.id;
  }, [me, detail.data?.occupant]);

  // displacement toast — server emits interlocking.occupantChanged
  // with reason:"displaced" when we get pushed out; show a transient
  // banner so the user understands the page suddenly says "vacant".
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
      // We were the previous occupant and got displaced.
      if (
        data.reason === "displaced" &&
        me &&
        !data.occupant?.userId &&
        isOccupying
      ) {
        setDisplacedToast(true);
      }
      // The replacement occupant arrived (someone forced us out).
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

  // ----- Displacement confirmation flow -----
  // When the box is already staffed by someone else, POST /join
  // returns 409. We surface the incumbent's login in a dialog and
  // only retry with force:true on explicit confirmation (§6.3d).
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
          // 409 — pop the dialog if we still have an occupant to name.
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

  // ----- Navigation guard while occupying -----
  // The router blocks transitions when isOccupying is true; the user
  // is asked whether to leave the box. Closing the browser tab is
  // NOT intercepted (the session lingers until explicit leave,
  // displacement or logout — §6.3d).
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
  const canActAsSignalman = !!me?.isSignalman;
  const busy = join.isPending || leave.isPending;

  return (
    <Container maxWidth="md" sx={{ py: { xs: 3, sm: 4 } }}>
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

            {canActAsSignalman ? (
              <Stack direction="row" spacing={1}>
                {isOccupying ? (
                  <Button
                    variant="contained"
                    color="warning"
                    startIcon={<LogoutIcon />}
                    onClick={handleLeave}
                    disabled={busy}
                  >
                    {t("interlocking:view.actions.leave")}
                  </Button>
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

        <Paper variant="outlined" sx={{ p: { xs: 2, sm: 3 } }}>
          <Typography variant="h6" gutterBottom>
            {t("interlocking:view.radioTitle")}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t("interlocking:view.radioComingSoon")}
          </Typography>
        </Paper>
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

  // Close icon for accessibility on touch devices where tapping the
  // dim overlay is not always intuitive.
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
