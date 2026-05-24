import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import { ApiError, apiFetch } from "./client";

// These mirror the JSON shapes emitted by pkgs/server/http/auth.go.
// Once tygo wiring lands they will be auto-generated instead.

export type Role = "driver" | "signalman" | "admin";

export interface CurrentUser {
  id: number;
  login: string;
  role: Role;
}

export interface LoginRequest {
  login: string;
  pin: string;
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
