import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "./client";

// LoginLayout is the trimmed shape returned by the public dropdown
// endpoint (`GET /api/v1/layouts/login`). The UI substitutes the
// `layout:system_default_label` i18n key for rows where
// `isSystem === true`, so the raw `name` value of the bootstrap row
// ("default") is never shown to the user.
export interface LoginLayout {
  id: number;
  name: string;
  isSystem: boolean;
}

// Layout is the canonical admin-view shape returned by
// `GET /api/v1/layouts` and the create/update/lock endpoints. The
// `commandStations` array promised by §4.1 will be added once command
// stations land — for now the wire shape mirrors the backend exactly.
export interface Layout {
  id: number;
  name: string;
  isSystem: boolean;
  locked: boolean;
  maxVehiclesPerUser: number;
}

const loginLayoutsQueryKey = ["layouts", "login"] as const;
const layoutsQueryKey = ["layouts"] as const;

export const DEFAULT_LAYOUT_MAX_VEHICLES_PER_USER = 8;

// useLoginLayouts powers the layout dropdown on the login page. It is
// unauthenticated (the matching backend route lives outside
// RequireAuth), so the hook is safe to mount before any session
// exists. The list is short and rarely changes, so a 30 s stale
// window comfortably outlasts a typical login flow without forcing a
// re-fetch on every keystroke.
export function useLoginLayouts() {
  return useQuery({
    queryKey: loginLayoutsQueryKey,
    queryFn: () => apiFetch<LoginLayout[]>("/api/v1/layouts/login"),
    staleTime: 30 * 1000,
    retry: false,
  });
}

// useAdminLayouts loads the full layout catalogue (including locked
// rows) for the admin management screen.
export function useAdminLayouts() {
  return useQuery({
    queryKey: layoutsQueryKey,
    queryFn: () => apiFetch<Layout[]>("/api/v1/layouts"),
    staleTime: 5 * 1000,
  });
}

// useCreateLayout sends POST /api/v1/layouts and invalidates the
// admin list so the new row appears immediately. The login dropdown
// query is also invalidated because a freshly-created (non-locked)
// layout should appear there on the next page load.
//
// `adminPin` is optional (§7a.7). Empty / omitted means "seed with
// the well-known default" — same UX as the system layout. The
// backend validates the digit / length policy.
export function useCreateLayout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      name: string;
      interlockingIds?: number[];
      commandStationIds?: number[];
      adminPin?: string;
      maxVehiclesPerUser?: number;
    }) =>
      apiFetch<Layout>("/api/v1/layouts", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: layoutsQueryKey });
      void qc.invalidateQueries({ queryKey: loginLayoutsQueryKey });
    },
  });
}

// useUpdateLayout sends PUT /api/v1/layouts/{id}. `adminPin` is
// optional — leaving it blank in the dialog MUST keep the existing
// digest, so the helper omits the field from the body when it is
// empty/undefined (no over-the-wire signal that the field even
// exists).
export function useUpdateLayout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: {
      id: number;
      name: string;
      interlockingIds?: number[];
      commandStationIds?: number[];
      adminPin?: string;
      maxVehiclesPerUser?: number;
    }) => {
      const body: Record<string, unknown> = {
        name: args.name,
        interlockingIds: args.interlockingIds,
      };
      if (args.commandStationIds !== undefined) {
        body.commandStationIds = args.commandStationIds;
      }
      if (args.adminPin && args.adminPin.length > 0) {
        body.adminPin = args.adminPin;
      }
      if (args.maxVehiclesPerUser !== undefined) {
        body.maxVehiclesPerUser = args.maxVehiclesPerUser;
      }
      return apiFetch<Layout>(`/api/v1/layouts/${args.id}`, {
        method: "PUT",
        body: JSON.stringify(body),
      });
    },
    onSuccess: (_data, args) => {
      void qc.invalidateQueries({ queryKey: layoutsQueryKey });
      void qc.invalidateQueries({ queryKey: loginLayoutsQueryKey });
      void qc.invalidateQueries({
        queryKey: ["layouts", args.id, "interlockings"],
      });
      void qc.invalidateQueries({
        queryKey: ["layouts", args.id, "command-stations"],
      });
    },
  });
}

export function useDeleteLayout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/v1/layouts/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: layoutsQueryKey });
      void qc.invalidateQueries({ queryKey: loginLayoutsQueryKey });
    },
  });
}

// useSetLayoutLock toggles the lock flag. POST /lock + DELETE /lock
// share the same hook because the contract is symmetrical: pass
// `lock: true` to lock, `lock: false` to unlock. Both branches
// return the updated Layout so the cache can be replaced atomically.
export function useSetLayoutLock() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { id: number; lock: boolean }) =>
      apiFetch<Layout>(`/api/v1/layouts/${args.id}/lock`, {
        method: args.lock ? "POST" : "DELETE",
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: layoutsQueryKey });
      void qc.invalidateQueries({ queryKey: loginLayoutsQueryKey });
    },
  });
}
