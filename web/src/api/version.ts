import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "./client";

export interface VersionInfo {
  version: string;
  tagCommit: string;
  buildCommit: string;
  buildTime: string;
}

const versionQueryKey = ["version"] as const;

export function useVersion() {
  return useQuery({
    queryKey: versionQueryKey,
    queryFn: () => apiFetch<VersionInfo>("/api/v1/version"),
    staleTime: 60 * 1000,
    retry: 1,
  });
}
