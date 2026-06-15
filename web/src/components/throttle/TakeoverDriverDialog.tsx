import { useEffect, useState } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  LinearProgress,
} from "@mui/material";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";

import type { TakeoverRequestedEvent } from "../../api/takeover";
import { useTakeoverActions } from "../../api/takeover";
import { layoutVehiclesQueryKey } from "../../api/vehicles";
import { useSocket } from "../../context/SocketContext";

interface TakeoverDriverDialogProps {
  pending: TakeoverRequestedEvent | null;
  onDismiss: () => void;
}

// TakeoverDriverDialog shows the 15-second reject window for drivers.
export function TakeoverDriverDialog({ pending, onDismiss }: TakeoverDriverDialogProps) {
  const { t } = useTranslation(["throttle", "interlocking"]);
  const { rejectTakeover } = useTakeoverActions();
  const [busy, setBusy] = useState(false);

  if (!pending) {
    return null;
  }

  return (
    <Dialog open onClose={onDismiss}>
      <DialogTitle>{t("throttle:takeover.requestedTitle")}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 2 }}>
          {t("throttle:takeover.requestedMessage", {
            login: pending.signalman.login,
            target: pending.target,
          })}
        </DialogContentText>
        <CountdownBar autoGrantAt={pending.autoGrantAt} />
      </DialogContent>
      <DialogActions>
        <Button
          color="error"
          disabled={busy}
          onClick={() => {
            setBusy(true);
            void rejectTakeover(pending.requestId).finally(() => {
              setBusy(false);
              onDismiss();
            });
          }}
        >
          {t("throttle:takeover.reject")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function CountdownBar({ autoGrantAt }: { autoGrantAt: number }) {
  const { t } = useTranslation("throttle");
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 200);
    return () => window.clearInterval(id);
  }, []);
  const total = 15_000;
  const remaining = Math.max(0, autoGrantAt - now);
  const pct = Math.min(100, ((total - remaining) / total) * 100);
  const sec = Math.ceil(remaining / 1000);
  return (
    <>
      <LinearProgress variant="determinate" value={pct} sx={{ mb: 1 }} />
      {t("takeover.countdown", { seconds: sec })}
    </>
  );
}

// useTakeoverDriverSession handles inbound takeover events on throttle.
export function useTakeoverDriverSession(layoutId: number | null) {
  const { subscribe } = useSocket();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [pending, setPending] = useState<TakeoverRequestedEvent | null>(null);
  const [evictionToast, setEvictionToast] = useState(false);

  useEffect(() => {
    return subscribe("takeover.requested", (payload) => {
      setPending(payload as TakeoverRequestedEvent);
    });
  }, [subscribe]);

  useEffect(() => {
    const clearPending = () => setPending(null);
    const u1 = subscribe("takeover.rejected", clearPending);
    const u2 = subscribe("takeover.cancelled", clearPending);
    const u3 = subscribe("takeover.granted", () => {
      clearPending();
      setEvictionToast(true);
      if (layoutId != null) {
        void qc.invalidateQueries({ queryKey: layoutVehiclesQueryKey(layoutId) });
      }
      window.setTimeout(() => navigate("/"), 1200);
    });
    return () => {
      u1();
      u2();
      u3();
    };
  }, [subscribe, layoutId, qc, navigate]);

  useEffect(() => {
    return subscribe("takeover.released", () => {
      if (layoutId != null) {
        void qc.invalidateQueries({ queryKey: layoutVehiclesQueryKey(layoutId) });
      }
    });
  }, [subscribe, layoutId, qc]);

  return {
    pending,
    dismissPending: () => setPending(null),
    evictionToast,
    clearEvictionToast: () => setEvictionToast(false),
  };
}
