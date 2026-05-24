// apiFetch is the single fetch wrapper used by the rest of the
// frontend. It centralises three concerns that would otherwise have
// to be repeated at every call site:
//
//   1. The API base URL — empty in development (the Vite dev server
//      reverse-proxies /api/* to the Go backend) and configurable at
//      build time via VITE_API_BASE for non-trivial deployments.
//   2. Cookie credentials — every request includes the
//      bigfred_session cookie so the backend can identify the caller.
//   3. JSON envelope handling — the backend returns {error: "code"}
//      for 4xx/5xx responses; we surface it as ApiError so React
//      Query can render a useful message.

export class ApiError extends Error {
  public readonly status: number;
  public readonly code: string;

  constructor(status: number, code: string) {
    super(`API ${status}: ${code}`);
    this.status = status;
    this.code = code;
  }
}

const API_BASE = (import.meta.env.VITE_API_BASE ?? "") as string;

export async function apiFetch<T = unknown>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type") && init.body != null) {
    headers.set("Content-Type", "application/json");
  }
  headers.set("Accept", "application/json");

  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    // Both same-origin (production single binary) and cross-origin
    // (Vite dev server proxy uses changeOrigin) request the cookie
    // via include — the backend echoes Access-Control-Allow-Credentials.
    credentials: "include",
  });

  if (res.status === 204) {
    return undefined as T;
  }

  // Try to parse JSON even on errors so we can extract the error code.
  let body: unknown = null;
  const text = await res.text();
  if (text.length > 0) {
    try {
      body = JSON.parse(text);
    } catch {
      body = text;
    }
  }

  if (!res.ok) {
    const code =
      typeof body === "object" && body !== null && "error" in body
        ? String((body as { error: unknown }).error)
        : `http_${res.status}`;
    throw new ApiError(res.status, code);
  }

  return body as T;
}
