import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { useMe } from "../api/auth";
import {
  useGrantSignalman,
  useRequestSudo,
  useRevokeSignalman,
  useRevokeSudo,
} from "../api/sudo";
import { useSocket } from "../context/SocketContext";

// useElevationListener subscribes to the `auth.elevationChanged` WS
// event and invalidates the `useMe` cache so every component reading
// effective role / sudo state re-renders on either change. Mounted
// once near the AppBar; the actual UI lives in the indicator
// components below.
export function useElevationListener(): void {
  const qc = useQueryClient();
  const socket = useSocket();
  useEffect(() => {
    return socket.subscribe("auth.elevationChanged", () => {
      void qc.invalidateQueries({ queryKey: ["auth", "me"] });
    });
  }, [socket, qc]);
}

interface SudoState {
  active: boolean;
  expiresAt: Date | null;
  request: (pin: string) => Promise<void>;
  revoke: () => Promise<void>;
  isPending: boolean;
}

// useSudoElevation drives the AppBar padlock — admin elevation that
// auto-expires after `cfg.SudoTTL` (default 2 min). The hook derives
// `active`/`expiresAt` from the `useMe` cache (single source of
// truth) and arms a self-fired timeout so the cache is also
// re-fetched if the WS broadcast for expiry is missed (transient
// drop).
export function useSudoElevation(): SudoState {
  const me = useMe().data;
  const qc = useQueryClient();
  const requestMutation = useRequestSudo();
  const revokeMutation = useRevokeSudo();

  const grant = me?.sudo ?? null;
  const expiresAt = grant ? new Date(grant.expiresAt) : null;
  const active = !!grant && (expiresAt?.getTime() ?? 0) > Date.now();

  useEffect(() => {
    if (!active || !expiresAt) return;
    const ms = expiresAt.getTime() - Date.now();
    if (ms <= 0) {
      void qc.invalidateQueries({ queryKey: ["auth", "me"] });
      return;
    }
    const id = window.setTimeout(() => {
      void qc.invalidateQueries({ queryKey: ["auth", "me"] });
    }, ms + 250);
    return () => window.clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, expiresAt?.getTime(), qc]);

  const layoutId = me?.layoutId ?? 0;

  return {
    active,
    expiresAt,
    request: async (pin: string) => {
      if (!layoutId) return;
      await requestMutation.mutateAsync({ layoutId, pin });
    },
    revoke: async () => {
      if (!layoutId) return;
      await revokeMutation.mutateAsync({ layoutId });
    },
    isPending: requestMutation.isPending || revokeMutation.isPending,
  };
}

interface SignalmanState {
  active: boolean;
  request: (pin: string) => Promise<void>;
  revoke: () => Promise<void>;
  isPending: boolean;
}

// useSignalmanGrant drives the engineer's-cap icon. Unlike sudo this
// is a PERMANENT layout-scoped self-grant — once accepted the user
// keeps the signalman role inside the layout until they (or an
// admin) revoke it. The active flag mirrors `me.isSignalman` while
// excluding admin-derived signalman authority, since the icon should
// only light up when the *self-grant* row exists.
//
// We approximate that distinction with `effectiveRole === "signalman"`:
// permanent admins (and sudo admins) have effectiveRole "admin" so
// the indicator stays idle for them — they don't need the icon.
export function useSignalmanGrant(): SignalmanState {
  const me = useMe().data;
  const grantMutation = useGrantSignalman();
  const revokeMutation = useRevokeSignalman();

  const layoutId = me?.layoutId ?? 0;
  const active = !!me && me.effectiveRole === "signalman";

  return {
    active,
    request: async (pin: string) => {
      if (!layoutId) return;
      await grantMutation.mutateAsync({ layoutId, pin });
    },
    revoke: async () => {
      if (!layoutId) return;
      await revokeMutation.mutateAsync({ layoutId });
    },
    isPending: grantMutation.isPending || revokeMutation.isPending,
  };
}
