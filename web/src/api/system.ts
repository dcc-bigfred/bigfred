import { apiFetch, ApiError } from "./client";

export interface SystemInfo {
  mode: string;
  canShutdown: boolean;
}

export type SystemShutdownMode = "poweroff" | "reboot";

export function fetchSystemInfo(): Promise<SystemInfo> {
  return apiFetch<SystemInfo>("/api/v1/admin/system");
}

/**
 * Fire-and-forget host shutdown.
 * Treats connection drop after send as success (server stops during shutdown).
 * Real HTTP error responses (ApiError with status) are rethrown.
 */
export async function requestSystemShutdown(
  mode: SystemShutdownMode,
): Promise<void> {
  try {
    await apiFetch("/api/v1/admin/system/shutdown", {
      method: "POST",
      body: JSON.stringify({ mode }),
    });
  } catch (err) {
    // Structured API errors must surface (503/409/400/…).
    if (err instanceof ApiError) {
      throw err;
    }
    // bigfred is stopped early during microinit stop_all — connection drop is expected.
    if (err instanceof TypeError) {
      return;
    }
    const status = (err as { status?: number })?.status;
    // status 0 / missing: aborted fetch / opaque network failure after send.
    if (status === undefined || status === 0) {
      return;
    }
    throw err;
  }
}
