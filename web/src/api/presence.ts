import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "./client";
import type { Role } from "./auth";
import { useSocket } from "../context/SocketContext";

export interface OccupiedInterlocking {
  id: number;
  name: string;
}

export interface PresenceUser {
  userId: number;
  login: string;
  organization: string;
  role: Role;
  occupiedInterlocking?: OccupiedInterlocking;
}

export function presenceQueryKey(layoutId: number) {
  return ["layouts", layoutId, "presence"] as const;
}

// Triggers supervisord rebuild when command-station attachments change
// (§7e.6). Uses the same GET as the dashboard presence poll, without
// subscribing to live presence updates.
export function useLayoutSupervisordSync(layoutId: number | null) {
  useQuery({
    queryKey: presenceQueryKey(layoutId ?? 0),
    queryFn: () =>
      apiFetch<PresenceUser[]>(`/api/v1/layouts/${layoutId}/presence`),
    enabled: layoutId != null && layoutId > 0,
    staleTime: 2 * 1000,
  });
}

export function useLayoutPresence(layoutId: number | null) {
  const qc = useQueryClient();
  const { subscribe } = useSocket();

  const query = useQuery({
    queryKey: presenceQueryKey(layoutId ?? 0),
    queryFn: () =>
      apiFetch<PresenceUser[]>(`/api/v1/layouts/${layoutId}/presence`),
    enabled: layoutId != null && layoutId > 0,
    staleTime: 2 * 1000,
  });

  useEffect(() => {
    if (layoutId == null || layoutId <= 0) return;
    return subscribe("layout.presenceChanged", (payload) => {
      const data = payload as { layoutId?: number; users?: PresenceUser[] };
      if (data.layoutId !== layoutId || !data.users) return;
      qc.setQueryData(presenceQueryKey(layoutId), data.users);
    });
  }, [layoutId, subscribe, qc]);

  return query;
}
