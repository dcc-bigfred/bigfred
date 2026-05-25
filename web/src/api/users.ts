import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "./client";
import type { Role } from "./auth";
import type { DCCAddressRange } from "./vehicles";

// User mirrors the JSON shape emitted by `pkgs/server/http/users.go`.
// `pinHash` is deliberately absent: the plaintext PIN never leaves the
// backend and the hash itself is not actionable on the client.
export interface User {
  id: number;
  login: string;
  role: Role;
  active: boolean;
  dccPool: DCCAddressRange[];
  createdAt: string;
  updatedAt: string;
}

export interface DccPoolRangeBody {
  from: number;
  to: number;
}

// USER_MANAGEABLE_ROLES is the closed list of permanent roles an
// admin can assign through the UI (§7a.2). `signalman` is excluded
// because it is a layout-scoped grant, not a permanent role.
export const USER_MANAGEABLE_ROLES: Role[] = ["driver", "admin"];

const usersQueryKey = ["users"] as const;

// useUsers loads the full user catalogue (admin view).
export function useUsers() {
  return useQuery({
    queryKey: usersQueryKey,
    queryFn: () => apiFetch<User[]>("/api/v1/users"),
    staleTime: 5 * 1000,
  });
}

export interface UserCreateBody {
  login: string;
  pin: string;
  role: Role;
  dccPool: DccPoolRangeBody[];
}

// useCreateUser sends POST /api/v1/users and invalidates the cached
// list so the new row appears immediately.
export function useCreateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: UserCreateBody) =>
      apiFetch<User>("/api/v1/users", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: usersQueryKey });
    },
  });
}

export interface UserUpdateBody {
  id: number;
  login?: string;
  role?: Role;
  // pin is optional; an empty / undefined value leaves the hash alone
  // (the backend coerces an empty string to "leave alone" as well).
  pin?: string;
  dccPool?: DccPoolRangeBody[];
}

export function useUpdateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: UserUpdateBody) => {
      const payload: Record<string, unknown> = {};
      if (body.login !== undefined) payload.login = body.login;
      if (body.role !== undefined) payload.role = body.role;
      if (body.pin !== undefined && body.pin !== "") payload.pin = body.pin;
      if (body.dccPool !== undefined) payload.dccPool = body.dccPool;
      return apiFetch<User>(`/api/v1/users/${body.id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      });
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: usersQueryKey });
    },
  });
}

// useSetUserActive flips the active flag. POST /activate / /deactivate
// share a single hook because the contract is symmetrical.
export function useSetUserActive() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { id: number; active: boolean }) =>
      apiFetch<User>(
        `/api/v1/users/${args.id}/${args.active ? "activate" : "deactivate"}`,
        { method: "POST" },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: usersQueryKey });
    },
  });
}

export function useDeleteUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/v1/users/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: usersQueryKey });
    },
  });
}
