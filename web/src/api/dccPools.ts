import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "./client";
import type { DCCAddressRange } from "./vehicles";

/** One user + declared DCC ranges from GET /api/v1/dcc-pools. */
export interface DCCPoolOverview {
  userId: number;
  login: string;
  organization: string;
  dccPool: DCCAddressRange[];
}

const dccPoolsQueryKey = ["dcc-pools"] as const;

/** Loads every user's DCC address pool (any authenticated caller). */
export function useDccPools() {
  return useQuery({
    queryKey: dccPoolsQueryKey,
    queryFn: () => apiFetch<DCCPoolOverview[]>("/api/v1/dcc-pools"),
    staleTime: 15 * 1000,
  });
}
