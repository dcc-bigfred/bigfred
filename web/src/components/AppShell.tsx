import { useMemo } from "react";
import { AppBar, Box, Chip, Stack, Toolbar, Tooltip, Typography } from "@mui/material";
import PeopleIcon from "@mui/icons-material/People";
import MapIcon from "@mui/icons-material/Map";
import EventIcon from "@mui/icons-material/Event";
import AccountTreeIcon from "@mui/icons-material/AccountTree";
import HistoryIcon from "@mui/icons-material/History";
import PersonIcon from "@mui/icons-material/Person";
import VpnKeyIcon from "@mui/icons-material/VpnKey";
import LockResetIcon from "@mui/icons-material/LockReset";
import LogoutIcon from "@mui/icons-material/Logout";
import { Outlet, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useLogout, useMe } from "../api/auth";
import LanguageMenu from "./LanguageMenu";
import TopBarMenu, { type TopBarMenuItem } from "./TopBarMenu";

// AppShell renders the top app bar shared by every authenticated
// screen. The post-login pages render inside its <Outlet/>.
//
// Layout, left → right inside the AppBar:
//   [BigFred title]   ……spacer……   [Administration ▾] [Account ▾] [🌐]
//
// The dropdowns use the reusable TopBarMenu component so adding a new
// top-level menu (e.g. "Operations" once the throttle screens land)
// is a single `<TopBarMenu/>` call with a declarative items array.
//
// Per §6 of the spec this will eventually grow into AppBar + Drawer +
// Container for per-page navigation; the dropdowns here are
// orthogonal — they expose account-level and admin-level actions
// that don't belong in a side drawer.
export default function AppShell() {
  const me = useMe().data;
  const logout = useLogout();
  const navigate = useNavigate();

  // Namespaces:
  //   common — appName, nav.*, comingSoon
  //   role   — driver/signalman/admin labels (for the Account caption)
  //   layout — system_default_label substituted for the system row name
  const { t } = useTranslation(["common", "role", "layout"]);

  // Each stubbed-out menu item carries the milestone in which it
  // becomes real. We render this as a "Coming soon (Mn)" tooltip so
  // the menu doubles as a roadmap reminder while the implementation
  // catches up. Replace the tooltip + disabled flag with a real
  // onClick/Link as each feature ships.
  const comingSoon = (milestone: string) => t("comingSoon", { milestone });

  const administrationItems: TopBarMenuItem[] = useMemo(
    () => [
      {
        id: "users",
        label: t("nav.administration.users"),
        icon: <PeopleIcon fontSize="small" />,
        disabled: true,
        tooltip: comingSoon("M2"),
      },
      {
        id: "layouts",
        label: t("nav.administration.layouts"),
        icon: <MapIcon fontSize="small" />,
        onClick: () => navigate("/admin/layouts"),
      },
      {
        id: "parties",
        label: t("nav.administration.parties"),
        icon: <EventIcon fontSize="small" />,
        disabled: true,
        tooltip: comingSoon("M4"),
      },
      {
        id: "interlockings",
        label: t("nav.administration.interlockings"),
        icon: <AccountTreeIcon fontSize="small" />,
        disabled: true,
        tooltip: comingSoon("M5"),
      },
      { id: "divider-1", divider: true },
      {
        id: "audit-log",
        label: t("nav.administration.auditLog"),
        icon: <HistoryIcon fontSize="small" />,
        disabled: true,
        tooltip: comingSoon("M3"),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t],
  );

  const accountItems: TopBarMenuItem[] = useMemo(
    () => [
      {
        id: "profile",
        label: t("nav.account.profile"),
        icon: <PersonIcon fontSize="small" />,
        disabled: true,
        tooltip: comingSoon("M2"),
      },
      {
        id: "apiKeys",
        label: t("nav.account.apiKeys"),
        icon: <VpnKeyIcon fontSize="small" />,
        disabled: true,
        tooltip: comingSoon("M6"),
      },
      {
        id: "changePin",
        label: t("nav.account.changePin"),
        icon: <LockResetIcon fontSize="small" />,
        disabled: true,
        tooltip: comingSoon("M2"),
      },
      { id: "divider-1", divider: true },
      {
        id: "logout",
        label: t("nav.account.logout"),
        icon: <LogoutIcon fontSize="small" />,
        // Only real action in the bootstrap milestone; the rest
        // become live as their pages land.
        onClick: () => logout.mutate(),
        disabled: logout.isPending,
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, logout.isPending],
  );

  const isAdmin = me?.role === "admin";

  return (
    <Box sx={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <AppBar position="sticky">
        <Toolbar>
          <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
            {t("appName")}
          </Typography>

          <Stack direction="row" spacing={1} alignItems="center">
            {/* Active layout indicator. The session is pinned to a
                single layout for its entire lifetime (§7a.1), so the
                user must be able to see at a glance which one they
                are in — switching requires logging out + back in. The
                system row is shown via its i18n label, never its
                stored Name. */}
            {me && (
              <Tooltip title={t("layout:loginPicker.label")}>
                <Chip
                  size="small"
                  color="default"
                  icon={<MapIcon fontSize="small" />}
                  label={
                    me.layoutIsSystem
                      ? t("layout:system_default_label")
                      : me.layoutName
                  }
                  sx={{
                    bgcolor: "rgba(255,255,255,0.16)",
                    color: "inherit",
                    "& .MuiChip-icon": { color: "inherit" },
                  }}
                />
              </Tooltip>
            )}

            {me && isAdmin && (
              <TopBarMenu
                label={t("nav.administration.menuLabel")}
                items={administrationItems}
              />
            )}

            {me && (
              <TopBarMenu
                label={t("nav.account.menuLabel")}
                // Caption surfaces the active identity next to the
                // menu name, so the user always knows "who am I"
                // without opening the dropdown. Role goes through
                // the `role` namespace (graceful fallback to the
                // raw code if the catalogue is behind the backend).
                caption={`${me.login} · ${t(`role:${me.role}` as const, {
                  defaultValue: me.role,
                })}`}
                items={accountItems}
              />
            )}

            <LanguageMenu />
          </Stack>
        </Toolbar>
      </AppBar>
      <Box component="main" sx={{ flexGrow: 1 }}>
        <Outlet />
      </Box>
    </Box>
  );
}
