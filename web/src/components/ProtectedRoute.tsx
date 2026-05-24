import { CircularProgress, Stack } from "@mui/material";
import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";

// ProtectedRoute gates every <Route> below it on the presence of a
// valid session. The pattern is the standard React Router v6 layout
// route: parent decides whether to render the matching child page
// (via <Outlet/>) or redirect to /login.
//
// We pass the originally-requested URL through location.state.from
// so the login page can bounce the user back after authentication.
export default function ProtectedRoute() {
  const { data, isLoading } = useMe();
  const location = useLocation();
  const { t } = useTranslation("common");

  if (isLoading) {
    return (
      <Stack
        sx={{ minHeight: "100vh", justifyContent: "center", alignItems: "center" }}
        role="status"
        aria-label={t("loading")}
      >
        <CircularProgress aria-label={t("loading")} />
      </Stack>
    );
  }

  if (!data) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  return <Outlet />;
}
