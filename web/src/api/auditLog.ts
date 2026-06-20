import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "./client";

export interface AuditEntry {
  streamId: string;
  layoutId: number;
  actorId: number;
  actorLogin: string;
  /** i18n key — e.g. "audit_radio_stop" */
  msg: string;
  /** template variables resolved on the frontend */
  vars: Record<string, string>;
  /** unix milliseconds */
  occurredAt: number;
}

export interface AuditLogResponse {
  entries: AuditEntry[];
}

export const auditLogQueryKey = ["audit-log"] as const;

export function useAuditLog(limit = 200) {
  return useQuery({
    queryKey: [...auditLogQueryKey, limit],
    queryFn: () =>
      apiFetch<AuditLogResponse>(
        `/api/v1/audit-log?limit=${limit}`,
      ),
    // Manual refresh only — never stale-while-revalidate.
    staleTime: Infinity,
    gcTime: 0,
  });
}
