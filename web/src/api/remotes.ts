import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { apiFetch } from "./client";
import { useSocket } from "../context/SocketContext";

export const REMOTE_PROTOCOL_Z21 = "z21" as const;
export const REMOTE_PROTOCOL_WITHROTTLE = "withrottle" as const;

export interface RemoteVehicle {
  vehicleId: string;
  addr: number;
}

export interface RemotePendingPairing {
  protocol: string;
  pairingCV3?: number;
  pairingCV4?: number;
  pairingCode?: string;
  displayLabel: string;
  expiresAt: number;
  handsetBrakeSecs?: number;
}

export interface RemoteProtocolInfo {
  protocol: string;
  enabled: boolean;
}

export interface RemoteStatus {
  protocol?: string;
  paired: boolean;
  clientKey?: string;
  pairedAt?: number;
  lastSeenAt?: number;
  allowAllVehicles: boolean;
  allowedVehicles: RemoteVehicle[];
  pendingPairing?: RemotePendingPairing;
  handsetBrakeSecs?: number;
  availableProtocols: RemoteProtocolInfo[];
  z21ServerEnabled: boolean;
}

export interface RemotePairingResult {
  protocol: string;
  pairingCV3?: number;
  pairingCV4?: number;
  pairingCode?: string;
  displayLabel: string;
  expiresAt: number;
  handsetBrakeSecs: number;
  instructions: string;
}

export const REMOTE_HANDSET_BRAKE_SECS_DEFAULT = 6;
export const REMOTE_HANDSET_BRAKE_SECS_MIN = 6;
export const REMOTE_HANDSET_BRAKE_SECS_MAX = 60;

export function remoteStatusQueryKey(layoutId: number, csId: number) {
  return ["layouts", layoutId, "command-stations", csId, "remotes", "status"] as const;
}

export function remoteClientsQueryKey(layoutId: number, csId: number) {
  return [
    "layouts",
    layoutId,
    "command-stations",
    csId,
    "remotes",
    "clients",
  ] as const;
}

function remotesBase(layoutId: number, csId: number) {
  return `/api/v1/layouts/${layoutId}/command-stations/${csId}/remotes`;
}

export function useRemoteStatus(
  layoutId: number | null,
  csId: number | null,
) {
  return useQuery({
    queryKey: remoteStatusQueryKey(layoutId ?? 0, csId ?? 0),
    queryFn: () =>
      apiFetch<RemoteStatus>(`${remotesBase(layoutId!, csId!)}/status`),
    enabled: layoutId != null && layoutId > 0 && csId != null && csId > 0,
    staleTime: 2 * 1000,
    refetchInterval: 15 * 1000,
  });
}

export interface RemoteClient {
  clientKey: string;
  protocol?: string;
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

export interface RemoteClientsSnapshot {
  layoutId: number;
  commandStationId: number;
  ipStickiness: boolean;
  updatedAt: number;
  clients: RemoteClient[];
}

export function useRemoteClients(
  layoutId: number | null,
  csId: number | null,
) {
  const qc = useQueryClient();
  const { subscribe } = useSocket();

  const query = useQuery({
    queryKey: remoteClientsQueryKey(layoutId ?? 0, csId ?? 0),
    queryFn: () =>
      apiFetch<RemoteClientsSnapshot>(`${remotesBase(layoutId!, csId!)}/clients`),
    enabled: layoutId != null && layoutId > 0 && csId != null && csId > 0,
    staleTime: 2 * 1000,
  });

  useEffect(() => {
    if (layoutId == null || layoutId <= 0) return;
    return subscribe("remote.clientsChanged", (payload) => {
      const data = payload as RemoteClientsSnapshot;
      if (data.layoutId !== layoutId || data.commandStationId !== csId) return;
      qc.setQueryData(remoteClientsQueryKey(layoutId, csId ?? 0), data);
    });
  }, [layoutId, csId, subscribe, qc]);

  return query;
}

export function useStartRemotePairing(
  layoutId: number,
  csId: number,
  protocol: string,
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      vehicleIds?: string[];
      allowAllVehicles?: boolean;
      handsetBrakeSecs?: number;
    }) =>
      apiFetch<RemotePairingResult>(
        `${remotesBase(layoutId, csId)}/${protocol}/pairing`,
        {
          method: "POST",
          body: JSON.stringify(body),
        },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: remoteStatusQueryKey(layoutId, csId),
      });
    },
  });
}

export function useUpdateRemoteSession(layoutId: number, csId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      vehicleIds?: string[];
      allowAllVehicles?: boolean;
      clientKey?: string;
    }) =>
      apiFetch<RemoteStatus>(
        `${remotesBase(layoutId, csId)}/session${
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
        queryKey: remoteStatusQueryKey(layoutId, csId),
      });
    },
  });
}

export function useCancelRemotePairing(layoutId: number, csId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<void>(`${remotesBase(layoutId, csId)}/pairing`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: remoteStatusQueryKey(layoutId, csId),
      });
    },
  });
}

export function useUnpairRemote(layoutId: number, csId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (clientKey?: string) =>
      apiFetch<void>(
        `${remotesBase(layoutId, csId)}/session${
          clientKey ? `?clientKey=${encodeURIComponent(clientKey)}` : ""
        }`,
        { method: "DELETE" },
      ),
    onSuccess: () => {
      void qc.invalidateQueries({
        queryKey: remoteStatusQueryKey(layoutId, csId),
      });
      void qc.invalidateQueries({
        queryKey: remoteClientsQueryKey(layoutId, csId),
      });
    },
  });
}
