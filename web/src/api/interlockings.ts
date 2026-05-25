import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "./client";

export interface Interlocking {
  id: number;
  name: string;
  location: string;
}

const interlockingsQueryKey = ["interlockings"] as const;

export function useInterlockings() {
  return useQuery({
    queryKey: interlockingsQueryKey,
    queryFn: () => apiFetch<Interlocking[]>("/api/v1/interlockings"),
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
