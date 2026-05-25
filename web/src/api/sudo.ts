import { useMutation, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "./client";
import type { SudoElevation } from "./auth";

// useRequestSudo issues `POST /api/v1/layouts/{layoutId}/sudo` with
// the user-typed PIN. The success branch returns the persisted
// SudoElevation (grantedAt + expiresAt) so the UI can start its
// countdown immediately, without waiting for the WS event to
// round-trip.
//
// `useMe` is invalidated on success so the AppBar indicator picks
// up the fresh `sudo` slice on the next render.
export function useRequestSudo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number; pin: string }) =>
      apiFetch<SudoElevation>(`/api/v1/layouts/${args.layoutId}/sudo`, {
        method: "POST",
        body: JSON.stringify({ pin: args.pin }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
}

// useRevokeSudo deletes the admin elevation. Idempotent on the
// server — the UI uses it for the "click the open lock to relock"
// affordance.
export function useRevokeSudo() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number }) =>
      apiFetch<void>(`/api/v1/layouts/${args.layoutId}/sudo`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
}

// useGrantSignalman issues `POST /api/v1/layouts/{layoutId}/signalman`
// — a PERMANENT self-grant of the layout-scoped signalman role. The
// PIN gate is identical to the sudo path but the resulting grant has
// no TTL.
export function useGrantSignalman() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number; pin: string }) =>
      apiFetch<void>(`/api/v1/layouts/${args.layoutId}/signalman`, {
        method: "POST",
        body: JSON.stringify({ pin: args.pin }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
}

export function useGrantSignalmanToUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number; userId: number }) =>
      apiFetch<void>(`/api/v1/layouts/${args.layoutId}/signalmen`, {
        method: "POST",
        body: JSON.stringify({ userId: args.userId }),
      }),
    onSuccess: (_data, args) => {
      void qc.invalidateQueries({
        queryKey: ["layouts", args.layoutId, "presence"],
      });
    },
  });
}

// useRevokeSignalman drops the user's signalman grant in the layout.
// Idempotent.
export function useRevokeSignalman() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number }) =>
      apiFetch<void>(`/api/v1/layouts/${args.layoutId}/signalman`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
}
