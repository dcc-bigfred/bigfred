import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";

import { clearCachedSession } from "../api/auth";
import { onSessionExpired } from "../auth/sessionExpiry";

/**
 * Redirects to /login when a background channel (WebSocket) discovers
 * the session cookie is no longer valid.
 */
export function useSessionExpiryRedirect(): void {
  const navigate = useNavigate();
  const location = useLocation();
  const qc = useQueryClient();

  useEffect(() => {
    return onSessionExpired(() => {
      clearCachedSession(qc);
      navigate("/login", { replace: true, state: { from: location } });
    });
  }, [navigate, location, qc]);
}
