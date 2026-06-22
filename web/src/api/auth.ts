import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import { ApiError, apiFetch } from "./client";

// These mirror the JSON shapes emitted by pkgs/server/http/auth.go.
// Once tygo wiring lands they will be auto-generated instead.

export type Role = "driver" | "signalman" | "admin";

// SudoElevation mirrors the active admin sudo grant reported by
// /me (§7a.7). Null when no elevation is live. The UI drives the
// AppBar countdown purely from `expiresAt`.
export interface SudoElevation {
  grantedAt: string; // ISO-8601
  expiresAt: string; // ISO-8601
}

// CurrentUser mirrors `meResponse` in pkgs/server/http/auth.go. The
// layout fields are derived from the JWT and immutable for the
// lifetime of the session (§7a.1): the user must log out and log in
// again to switch layout.
export interface CurrentUser {
  id: number;
  login: string;
  organization: string;
  role: Role;
  /** effectiveRole resolves signalman grants and sudo for the active layout (§7a.2). */
  effectiveRole: Role;
  /**
   * isSignalman is true when the caller may operate as a signalman
   * in the active layout — admins (sudo or permanent) and layout
   * signalman grants both count.
   */
  isSignalman: boolean;
  active: boolean;
  createdAt: string; // ISO-8601
  updatedAt: string; // ISO-8601
  layoutId: number;
  layoutName: string;
  layoutIsSystem: boolean;
  /**
   * sudo carries the active admin elevation (§7a.7), or null when
   * none is live.
   */
  sudo: SudoElevation | null;
}

export interface ChangePinRequest {
  currentPin: string;
  newPin: string;
}

export interface UpdateProfileRequest {
  organization: string;
}

export interface LoginRequest {
  login: string;
  pin: string;
  layoutId: number;
}

const meQueryKey = ["auth", "me"] as const;

// useMe is the single source of truth for "who am I, if anyone?" in
// the React tree. It returns:
//   - data === undefined while loading
//   - data === null when the request returned 401 (no session)
//   - data === CurrentUser when authenticated
//
// Treating the 401 case as a successful `null` (instead of an error)
// removes ceremony at every call site: ProtectedRoute just checks
// for null/undefined.
export function useMe(): UseQueryResult<CurrentUser | null> {
  return useQuery({
    queryKey: meQueryKey,
    queryFn: async () => {
      try {
        return await apiFetch<CurrentUser>("/api/v1/auth/me");
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          return null;
        }
        throw err;
      }
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
  });
}

// useLogin returns a TanStack mutation that POSTs the login form,
// stores the resulting CurrentUser into the meQuery cache and
// triggers a re-render of every component reading useMe().
export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: LoginRequest) =>
      apiFetch<CurrentUser>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: (user) => {
      qc.setQueryData(meQueryKey, user);
    },
  });
}

// useLogout clears the cookie via /auth/logout and resets the cached
// identity so any in-flight render sees the user as signed out.
export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => apiFetch<void>("/api/v1/auth/logout", { method: "POST" }),
    onSuccess: () => {
      qc.setQueryData(meQueryKey, null);
    },
  });
}

// useChangePin rotates the caller's password after verifying the
// current one. On success the cached /me row is invalidated so
// updatedAt refreshes on the profile page.
export function useChangePin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ChangePinRequest) =>
      apiFetch<void>("/api/v1/auth/me/pin", {
        method: "PUT",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: meQueryKey });
    },
  });
}

// useUpdateProfile updates self-service profile fields and refreshes
// the cached /me row from the response body.
export function useUpdateProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateProfileRequest) =>
      apiFetch<CurrentUser>("/api/v1/auth/me/profile", {
        method: "PUT",
        body: JSON.stringify(body),
      }),
    onSuccess: (user) => {
      qc.setQueryData(meQueryKey, user);
    },
  });
}
