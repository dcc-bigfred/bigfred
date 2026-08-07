import { useQuery } from "@tanstack/react-query";

import { apiFetch, ApiError } from "./client";

export interface SystemInfo {
  mode: string;
  canShutdown: boolean;
}

export interface SystemPorts {
  osUI: boolean;
  grafana: boolean;
}

export interface MicroinitServiceStatus {
  name: string;
  state: string;
  pid?: number | null;
  restarts: number;
  enabled: boolean;
  liveness_failures?: number;
  labels?: Record<string, string>;
}

export interface MicroinitInfo {
  version: string;
  tag_commit?: string;
  build_commit?: string;
  build_time?: string;
  pid: number;
  hostname: string;
  uptime_secs: number;
  socket: string;
  mode: string;
  services_total: number;
  services_running: number;
  otel_enabled: boolean;
}

export type SystemShutdownMode = "poweroff" | "reboot";

export function fetchSystemInfo(): Promise<SystemInfo> {
  return apiFetch<SystemInfo>("/api/v1/admin/system");
}

export async function requestSystemShutdown(
  mode: SystemShutdownMode,
): Promise<void> {
  try {
    await apiFetch("/api/v1/admin/system/shutdown", {
      method: "POST",
      body: JSON.stringify({ mode }),
    });
  } catch (err) {
    if (err instanceof ApiError) throw err;
    if (err instanceof TypeError) return;
    const status = (err as { status?: number })?.status;
    if (status === undefined || status === 0) return;
    throw err;
  }
}

export function useSystemInfo() {
  return useQuery({
    queryKey: ["admin", "system"],
    queryFn: fetchSystemInfo,
    staleTime: 5 * 1000,
  });
}

export function useSystemPorts() {
  return useQuery({
    queryKey: ["admin", "system", "ports"],
    queryFn: () => apiFetch<SystemPorts>("/api/v1/admin/system/ports"),
    staleTime: 5 * 1000,
    refetchInterval: 15 * 1000,
  });
}

export function useMicroinitServices() {
  return useQuery({
    queryKey: ["admin", "microinit", "services"],
    queryFn: async () => {
      const res = await apiFetch<{ services: MicroinitServiceStatus[] }>(
        "/api/v1/admin/microinit/services",
      );
      return res.services ?? [];
    },
    staleTime: 2 * 1000,
    refetchInterval: 5 * 1000,
  });
}

export function useMicroinitInfo() {
  return useQuery({
    queryKey: ["admin", "microinit", "info"],
    queryFn: () => apiFetch<MicroinitInfo>("/api/v1/admin/microinit/info"),
    staleTime: 5 * 1000,
    refetchInterval: 10 * 1000,
  });
}
