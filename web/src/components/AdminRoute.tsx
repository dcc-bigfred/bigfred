import { Navigate, Outlet } from "react-router-dom";

import { useMe } from "../api/auth";

// AdminRoute is a layout route that only renders its <Outlet/> when
// the current user is an effective admin in the active layout
// (permanent or sudo, §7a.7). Anybody else is bounced to the home
// screen so a hand-typed URL cannot reach an admin-only page (the
// backend still enforces the same check — RequireRole consults
// AuthService.Effective — so this is a UX shortcut, not a security
// boundary).
//
// `useMe` is already cached by the surrounding ProtectedRoute, so
// reading it here is effectively free; the hook also returns
// `undefined` while in flight, but ProtectedRoute renders a spinner
// in that state and only mounts AdminRoute once `data` is non-null.
export default function AdminRoute() {
  const me = useMe().data;
  if (!me || me.effectiveRole !== "admin") {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}
