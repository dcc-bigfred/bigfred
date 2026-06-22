import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "./client";
import type { TakeoverTarget } from "./takeover";

export interface LeaseEntry {
  kind: TakeoverTarget;
  targetId: string;
  targetName: string;
  fromUserId: number;
  fromLogin: string;
  toUserId: number;
  toLogin: string;
  expiresAt: string;
  speedLimit: number;
}

export interface LendableTarget {
  kind: TakeoverTarget;
  targetId: string;
  targetName: string;
}

export interface LendableUser {
  userId: number;
  login: string;
  organization?: string;
}

export interface LendableCatalogue {
  targets: LendableTarget[];
  users: LendableUser[];
}

export const leasesQueryKey = {
  received: ["leases", "received"] as const,
  granted: ["leases", "granted"] as const,
  lendable: ["leases", "lendable"] as const,
};

export function useReceivedLeases() {
  return useQuery({
    queryKey: leasesQueryKey.received,
    queryFn: () => apiFetch<LeaseEntry[]>("/api/v1/leases/received"),
    refetchInterval: 30_000,
  });
}

export function useGrantedLeases() {
  return useQuery({
    queryKey: leasesQueryKey.granted,
    queryFn: () => apiFetch<LeaseEntry[]>("/api/v1/leases/granted"),
    refetchInterval: 30_000,
  });
}

export function useLendable() {
  return useQuery({
    queryKey: leasesQueryKey.lendable,
    queryFn: () => apiFetch<LendableCatalogue>("/api/v1/leases/lendable"),
  });
}

export function useCreateLease() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      kind: TakeoverTarget;
      targetId: string;
      toUserId: number;
      speedLimit: number;
      durationSeconds: number;
    }) =>
      apiFetch<LeaseEntry>("/api/v1/leases", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["leases"] });
    },
  });
}

export function useUpdateLease() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      kind,
      targetId,
      ...body
    }: {
      kind: TakeoverTarget;
      targetId: string;
      speedLimit?: number;
      durationSeconds?: number;
    }) =>
      apiFetch<LeaseEntry>(`/api/v1/leases/${kind}/${encodeURIComponent(targetId)}`, {
        method: "PATCH",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["leases"] });
    },
  });
}

export function useRevokeLease() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      kind,
      targetId,
    }: {
      kind: TakeoverTarget;
      targetId: string;
    }) =>
      apiFetch<void>(`/api/v1/leases/${kind}/${encodeURIComponent(targetId)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["leases"] });
    },
  });
}

export function invalidateLeases(qc: ReturnType<typeof useQueryClient>) {
  void qc.invalidateQueries({ queryKey: ["leases"] });
}
