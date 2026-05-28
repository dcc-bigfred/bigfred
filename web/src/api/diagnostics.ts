import { useQuery } from "@tanstack/react-query";

import { apiFetch } from "./client";

export interface DiagnosticEntry {
  id: string;
  label: string;
}

export interface DiagnosticGroup {
  id: string;
  label: string;
  entries: DiagnosticEntry[];
}

export interface DiagnosticSources {
  groups: DiagnosticGroup[];
}

export interface DiagnosticContent {
  fileId: string;
  fileName: string;
  size: number;
  truncated: boolean;
  content: string;
}

const sourcesQueryKey = ["diagnostics", "sources"] as const;

export function useDiagnosticSources() {
  return useQuery({
    queryKey: sourcesQueryKey,
    queryFn: () => apiFetch<DiagnosticSources>("/api/v1/diagnostics/sources"),
    staleTime: 10 * 1000,
  });
}

export function fetchDiagnosticContent(
  fileId: string,
  tailLines = 500,
): Promise<DiagnosticContent> {
  const params = new URLSearchParams({
    fileId,
    tailLines: String(tailLines),
  });
  return apiFetch<DiagnosticContent>(
    `/api/v1/diagnostics/content?${params.toString()}`,
  );
}
