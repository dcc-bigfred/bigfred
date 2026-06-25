import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { apiFetch } from "./client";
import { useSocket } from "../context/SocketContext";

export interface Z21RemoteVehicle {
  vehicleId: string;
  addr: number;
}

export interface Z21RemotePendingPairing {
  pairingCV3: number;
  pairingCV4: number;
  displayLabel: string;
  expiresAt: number;
  handsetBrakeSecs?: number;
}

export interface Z21RemoteStatus {
  paired: boolean;
  clientKey?: string;
  pairedAt?: number;
  lastSeenAt?: number;
  allowAllVehicles: boolean;
  allowedVehicles: Z21RemoteVehicle[];
  pendingPairing?: Z21RemotePendingPairing;
  handsetBrakeSecs?: number;
  z21ServerEnabled: boolean;
}

export interface Z21RemotePairingResult {
  pairingCV3: number;
  pairingCV4: number;
  displayLabel: string;
  expiresAt: number;
  handsetBrakeSecs: number;
  instructions: string;
}

export const Z21_HANDSET_BRAKE_SECS_DEFAULT = 6;
export const Z21_HANDSET_BRAKE_SECS_MIN = 6;
export const Z21_HANDSET_BRAKE_SECS_MAX = 60;

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

export interface Z21RemoteClient {
  clientKey: string;
  ip: string;
  port: number;
  paired: boolean;
  userId?: number;
  userLogin?: string;
  lastSeenAt: number;
  connectedAt: number;
  sessionExpiresAt?: number;
  idleBraked: boolean;
}

export interface Z21RemoteClientsSnapshot {
  layoutId: number;
  commandStationId: number;
  ipStickiness: boolean;
  updatedAt: number;
  clients: Z21RemoteClient[];
}

export function z21RemoteClientsQueryKey(layoutId: number, csId: number) {
  return [
    "layouts",
    layoutId,
    "command-stations",
    csId,
    "z21-remote",
    "clients",
  ] as const;
}

export function useZ21RemoteClients(
  layoutId: number | null,
  csId: number | null,
) {
  const qc = useQueryClient();
  const { subscribe } = useSocket();

  const query = useQuery({
    queryKey: z21RemoteClientsQueryKey(layoutId ?? 0, csId ?? 0),
    queryFn: () =>
      apiFetch<Z21RemoteClientsSnapshot>(
        `/api/v1/layouts/${layoutId}/command-stations/${csId}/z21-remote/clients`,
      ),
    enabled: layoutId != null && layoutId > 0 && csId != null && csId > 0,
    staleTime: 2 * 1000,
  });

  useEffect(() => {
    if (layoutId == null || layoutId <= 0) return;
    return subscribe("z21.clientsChanged", (payload) => {
      const data = payload as Z21RemoteClientsSnapshot;
      if (data.layoutId !== layoutId || data.commandStationId !== csId) return;
      qc.setQueryData(z21RemoteClientsQueryKey(layoutId, csId ?? 0), data);
    });
  }, [layoutId, csId, subscribe, qc]);

  return query;
}

export function useStartZ21Pairing(layoutId: number, csId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      vehicleIds?: string[];
      allowAllVehicles?: boolean;
      handsetBrakeSecs?: number;
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

export function useCancelZ21Pairing(layoutId: number, csId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<void>(
        `/api/v1/layouts/${layoutId}/command-stations/${csId}/z21-remote/pairing`,
        { method: "DELETE" },
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
      void qc.invalidateQueries({
        queryKey: z21RemoteClientsQueryKey(layoutId, csId),
      });
    },
  });
}
