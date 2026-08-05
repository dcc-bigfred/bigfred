import { apiFetch } from "./client";

export interface SystemInfo {
  mode: string;
  canShutdown: boolean;
}

export type SystemShutdownMode = "poweroff" | "reboot";

export function fetchSystemInfo(): Promise<SystemInfo> {
  return apiFetch<SystemInfo>("/api/v1/admin/system");
}

/** Fire-and-forget host shutdown. Network errors after send are treated as success. */
export async function requestSystemShutdown(
  mode: SystemShutdownMode,
): Promise<void> {
  try {
    await apiFetch("/api/v1/admin/system/shutdown", {
      method: "POST",
      body: JSON.stringify({ mode }),
    });
  } catch (err) {
    // bigfred is stopped early during microinit stop_all — connection drop is expected.
    if (err instanceof TypeError) {
      return;
    }
    const status = (err as { status?: number })?.status;
    if (status === undefined || status === 0) {
      return;
    }
    throw err;
  }
}
