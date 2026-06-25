import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "./client";

export interface Z21RemoteVehicle {
  vehicleId: string;
  addr: number;
}

export interface Z21RemotePendingPairing {
  pairingCV3: number;
  pairingCV4: number;
  displayLabel: string;
  expiresAt: number;
}

export interface Z21RemoteStatus {
  paired: boolean;
  clientKey?: string;
  pairedAt?: number;
  lastSeenAt?: number;
  allowAllVehicles: boolean;
  allowedVehicles: Z21RemoteVehicle[];
  pendingPairing?: Z21RemotePendingPairing;
  z21ServerEnabled: boolean;
}

export interface Z21RemotePairingResult {
  pairingCV3: number;
  pairingCV4: number;
  displayLabel: string;
  expiresAt: number;
  instructions: string;
}

export function z21RemoteQueryKey(layoutId: number, csId: number) {
  return ["layouts", layoutId, "command-stations", csId, "z21-remote"] as const;
}

export function useZ21RemoteStatus(
  layoutId: number | null,
  csId: number | null,
) {
  return useQuery({
    queryKey: z21RemoteQueryKey(layoutId ?? 0, csId ?? 0),
    queryFn: () =>
      apiFetch<Z21RemoteStatus>(
        `/api/v1/layouts/${layoutId}/command-stations/${csId}/z21-remote`,
      ),
    enabled: layoutId != null && layoutId > 0 && csId != null && csId > 0,
    staleTime: 2 * 1000,
    refetchInterval: 15 * 1000,
  });
}

export function useStartZ21Pairing(layoutId: number, csId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      vehicleIds?: string[];
      allowAllVehicles?: boolean;
    }) =>
      apiFetch<Z21RemotePairingResult>(
        `/api/v1/layouts/${layoutId}/command-stations/${csId}/z21-remote/pairing`,
        {
          method: "POST",
          body: JSON.stringify(body),
        },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: z21RemoteQueryKey(layoutId, csId),
      });
    },
  });
}

export function useUpdateZ21RemoteSession(layoutId: number, csId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      vehicleIds?: string[];
      allowAllVehicles?: boolean;
      clientKey?: string;
    }) =>
      apiFetch<Z21RemoteStatus>(
        `/api/v1/layouts/${layoutId}/command-stations/${csId}/z21-remote/session${
          body.clientKey
            ? `?clientKey=${encodeURIComponent(body.clientKey)}`
            : ""
        }`,
        {
          method: "PATCH",
          body: JSON.stringify({
            vehicleIds: body.vehicleIds,
            allowAllVehicles: body.allowAllVehicles,
          }),
        },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: z21RemoteQueryKey(layoutId, csId),
      });
    },
  });
}

export function useUnpairZ21Remote(layoutId: number, csId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (clientKey?: string) =>
      apiFetch<void>(
        `/api/v1/layouts/${layoutId}/command-stations/${csId}/z21-remote/session${
          clientKey ? `?clientKey=${encodeURIComponent(clientKey)}` : ""
        }`,
        { method: "DELETE" },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: z21RemoteQueryKey(layoutId, csId),
      });
    },
  });
}
