import { AppBar, Box, Button, Toolbar, Typography } from "@mui/material";
import { Outlet } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useLogout, useMe } from "../api/auth";
import LanguageMenu from "./LanguageMenu";

// AppShell renders the top app bar shared by every authenticated
// screen. The post-login pages render inside its <Outlet/>.
// Per §6 of the spec this will grow into a MUI AppBar + Drawer +
// Container as more pages land; for the bootstrap a simple bar with
// a language switcher and a Logout button is enough.
export default function AppShell() {
  const me = useMe().data;
  const logout = useLogout();

  // Three namespaces: `common` for the app title and Logout button,
  // `auth` for the role badge format, `role` for the localised role
  // names that map 1:1 to Go's domain.Role enum.
  const { t } = useTranslation(["common", "auth", "role"]);

  return (
    <Box sx={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <AppBar position="sticky">
        <Toolbar>
          <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
            {t("common:appName")}
          </Typography>

          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <LanguageMenu />

            {me && (
              <>
                <Typography variant="body2" sx={{ ml: 1 }}>
                  {t("auth:userBadge.withRole", {
                    login: me.login,
                    // Backend ships the role as a stable code
                    // ("driver" / "signalman" / "admin"); the
                    // `role` namespace turns it into a localised
                    // label. Falling back to the raw code keeps
                    // the UI usable if a new role is added on the
                    // backend before the catalogue catches up.
                    role: t(`role:${me.role}` as const, {
                      defaultValue: me.role,
                    }),
                  })}
                </Typography>
                <Button
                  color="inherit"
                  variant="outlined"
                  onClick={() => logout.mutate()}
                  disabled={logout.isPending}
                  sx={{ ml: 1 }}
                >
                  {t("common:actions.logout")}
                </Button>
              </>
            )}
          </Box>
        </Toolbar>
      </AppBar>
      <Box component="main" sx={{ flexGrow: 1 }}>
        <Outlet />
      </Box>
    </Box>
  );
}
