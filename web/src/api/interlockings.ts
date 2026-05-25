import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "./client";
import { useSocket } from "../context/SocketContext";

export interface InterlockingOccupant {
  userId: number;
  login: string;
}

export interface Interlocking {
  id: number;
  name: string;
  location: string;
  occupant?: InterlockingOccupant;
}

const interlockingsQueryKey = ["interlockings"] as const;
const interlockingsCatalogueQueryKey = ["interlockings", "catalogue"] as const;

export function useInterlockings() {
  return useQuery({
    queryKey: interlockingsQueryKey,
    queryFn: () => apiFetch<Interlocking[]>("/api/v1/interlockings"),
    staleTime: 2 * 1000,
  });
}

// useDashboardInterlockings loads layout-scoped interlockings for the
// home dashboard and merges interlocking.occupantChanged events.
export function useDashboardInterlockings() {
  const qc = useQueryClient();
  const { subscribe } = useSocket();
  const query = useInterlockings();

  useEffect(() => {
    return subscribe("interlocking.occupantChanged", (payload) => {
      const data = payload as {
        interlockingId?: number;
        occupant?: InterlockingOccupant;
      };
      if (data.interlockingId == null) return;
      qc.setQueryData<Interlocking[]>(interlockingsQueryKey, (prev) => {
        if (!prev) return prev;
        return prev.map((row) =>
          row.id === data.interlockingId
            ? { ...row, occupant: data.occupant ?? undefined }
            : row,
        );
      });
    });
  }, [subscribe, qc]);

  return query;
}

export function useInterlockingsCatalogue() {
  return useQuery({
    queryKey: interlockingsCatalogueQueryKey,
    queryFn: () =>
      apiFetch<Interlocking[]>("/api/v1/interlockings/catalogue"),
    staleTime: 5 * 1000,
  });
}

export function useCreateInterlocking() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string; location: string }) =>
      apiFetch<Interlocking>("/api/v1/interlockings", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: interlockingsCatalogueQueryKey });
      void qc.invalidateQueries({ queryKey: interlockingsQueryKey });
    },
  });
}

export function useUpdateInterlocking() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { id: number; name: string; location: string }) =>
      apiFetch<Interlocking>(`/api/v1/interlockings/${args.id}`, {
        method: "PUT",
        body: JSON.stringify({ name: args.name, location: args.location }),
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: interlockingsCatalogueQueryKey });
      void qc.invalidateQueries({ queryKey: interlockingsQueryKey });
    },
  });
}

export function useDeleteInterlocking() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      apiFetch<void>(`/api/v1/interlockings/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: interlockingsCatalogueQueryKey });
      void qc.invalidateQueries({ queryKey: interlockingsQueryKey });
    },
  });
}

export function layoutInterlockingsQueryKey(layoutId: number) {
  return ["layouts", layoutId, "interlockings"] as const;
}

export function useLayoutInterlockings(layoutId: number | null) {
  return useQuery({
    queryKey: layoutInterlockingsQueryKey(layoutId ?? 0),
    queryFn: () =>
      apiFetch<Interlocking[]>(`/api/v1/layouts/${layoutId}/interlockings`),
    enabled: layoutId != null && layoutId > 0,
    staleTime: 5 * 1000,
  });
}

export function useSetLayoutInterlockings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { layoutId: number; interlockingIds: number[] }) =>
      apiFetch<Interlocking[]>(
        `/api/v1/layouts/${args.layoutId}/interlockings`,
        {
          method: "PUT",
          body: JSON.stringify({ interlockingIds: args.interlockingIds }),
        },
      ),
    onSuccess: (_data, args) => {
      void qc.invalidateQueries({
        queryKey: layoutInterlockingsQueryKey(args.layoutId),
      });
    },
  });
}
