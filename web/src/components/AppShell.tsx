import { useEffect, useMemo, useState } from "react";
import {
  AppBar,
  Box,
  Button,
  Chip,
  IconButton,
  Stack,
  Toolbar,
  Tooltip,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import SpeedIcon from "@mui/icons-material/Speed";
import MenuIcon from "@mui/icons-material/Menu";
import PeopleIcon from "@mui/icons-material/People";
import MapIcon from "@mui/icons-material/Map";
import EventIcon from "@mui/icons-material/Event";
import AccountTreeIcon from "@mui/icons-material/AccountTree";
import DirectionsRailwayIcon from "@mui/icons-material/DirectionsRailway";
import HistoryIcon from "@mui/icons-material/History";
import BugReportIcon from "@mui/icons-material/BugReport";
import PersonIcon from "@mui/icons-material/Person";
import TrainIcon from "@mui/icons-material/Train";
import VpnKeyIcon from "@mui/icons-material/VpnKey";
import LockResetIcon from "@mui/icons-material/LockReset";
import LogoutIcon from "@mui/icons-material/Logout";
import { Link, Outlet, useMatch, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useLogout, useMe } from "../api/auth";
import { SocketProvider } from "../context/SocketContext";
import LanguageMenu from "./LanguageMenu";
import SudoIndicators from "./SudoIndicator";
import MobileNavDrawer, { type MobileNavSection } from "./MobileNavDrawer";
import TopBarMenu, { type TopBarMenuItem } from "./TopBarMenu";

// AppShell renders the top app bar shared by every authenticated
// screen. The post-login pages render inside its <Outlet/>.
//
// Layout, left → right inside the AppBar:
//   [☰] [BigFred] ……spacer…… [Sterowanie] [layout chip] [sudo] [menus] [🌐]
//
// Throttle is a persistent top-bar link (not inside the My dropdown).
// The dropdowns use TopBarMenu for account-level / admin-level actions.
//
// Per §6 of the spec this will eventually grow into AppBar + Drawer +
// Container for per-page navigation; the dropdowns here are
// orthogonal — they expose account-level and admin-level actions
// that don't belong in a side drawer.
export default function AppShell() {
  const me = useMe().data;
  const logout = useLogout();
  const navigate = useNavigate();
  const theme = useTheme();
  const isCompactNav = useMediaQuery(theme.breakpoints.down("md"));
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const onThrottlePage = Boolean(useMatch("/throttle"));

  useEffect(() => {
    if (!isCompactNav) setMobileNavOpen(false);
  }, [isCompactNav]);

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
        onClick: () => navigate("/admin/users"),
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
        onClick: () => navigate("/admin/interlockings"),
      },
      {
        id: "commandStations",
        label: t("nav.administration.commandStations"),
        icon: <DirectionsRailwayIcon fontSize="small" />,
        onClick: () => navigate("/admin/command-stations"),
      },
      {
        id: "diagnostics",
        label: t("nav.administration.diagnostics"),
        icon: <BugReportIcon fontSize="small" />,
        onClick: () => navigate("/admin/diagnostics"),
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

  const myItems: TopBarMenuItem[] = useMemo(
    () => [
      {
        id: "vehicles",
        label: t("nav.my.vehicles"),
        icon: <TrainIcon fontSize="small" />,
        onClick: () => navigate("/my/vehicles"),
      },
      {
        id: "trains",
        label: t("nav.my.trains"),
        icon: <DirectionsRailwayIcon fontSize="small" />,
        onClick: () => navigate("/my/trains"),
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

  const isAdmin = me?.effectiveRole === "admin";

  const mobileNavSections: MobileNavSection[] = useMemo(() => {
    if (!me) return [];
    const sections: MobileNavSection[] = [
      { id: "my", label: t("nav.my.menuLabel"), items: myItems },
    ];
    if (isAdmin) {
      sections.push({
        id: "administration",
        label: t("nav.administration.menuLabel"),
        items: administrationItems,
      });
    }
    sections.push({
      id: "account",
      label: t("nav.account.menuLabel"),
      items: accountItems,
    });
    return sections;
  }, [
    me,
    isAdmin,
    myItems,
    administrationItems,
    accountItems,
    t,
  ]);

  const accountCaption =
    me &&
    `${me.login} · ${t(`role:${me.effectiveRole}` as const, {
      defaultValue: me.effectiveRole,
    })}`;

  return (
    <SocketProvider enabled={!!me}>
    <Box sx={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <AppBar position="sticky">
        <Toolbar>
          {isCompactNav && me && (
            <IconButton
              color="inherit"
              edge="start"
              aria-label={t("nav.openMenu")}
              onClick={() => setMobileNavOpen(true)}
              sx={{ mr: 1 }}
            >
              <MenuIcon />
            </IconButton>
          )}

          <Typography
            variant="h6"
            component={Link}
            to="/"
            sx={{
              flexGrow: 1,
              color: "inherit",
              textDecoration: "none",
              "&:hover": { opacity: 0.9 },
              minWidth: 0,
            }}
            noWrap
          >
            {t("appName")}
          </Typography>

          <Stack
            direction="row"
            spacing={1}
            alignItems="center"
            sx={{ flexShrink: 0 }}
          >
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

            {/* Sudo indicators (§7a.7). The padlock drives the
                temporary admin elevation; the engineer-cap drives
                the permanent signalman self-grant. Both stay
                hidden until the user is authenticated. */}
            {me && <SudoIndicators />}

            {me && (
              <Button
                color="inherit"
                component={Link}
                to="/throttle"
                startIcon={<SpeedIcon />}
                aria-current={onThrottlePage ? "page" : undefined}
                sx={{
                  textTransform: "none",
                  fontWeight: onThrottlePage ? 700 : 500,
                  flexShrink: 0,
                }}
              >
                {t("nav.my.throttle")}
              </Button>
            )}

            {me && !isCompactNav && (
              <TopBarMenu label={t("nav.my.menuLabel")} items={myItems} />
            )}

            {me && isAdmin && !isCompactNav && (
              <TopBarMenu
                label={t("nav.administration.menuLabel")}
                items={administrationItems}
              />
            )}

            {me && !isCompactNav && (
              <TopBarMenu
                label={t("nav.account.menuLabel")}
                caption={accountCaption ?? undefined}
                items={accountItems}
              />
            )}

            <LanguageMenu />
          </Stack>
        </Toolbar>
      </AppBar>

      {me && isCompactNav && (
        <MobileNavDrawer
          open={mobileNavOpen}
          onClose={() => setMobileNavOpen(false)}
          title={t("nav.drawerTitle")}
          closeLabel={t("nav.closeMenu")}
          sections={mobileNavSections}
          identityLine={accountCaption ?? undefined}
        />
      )}

      <Box component="main" sx={{ flexGrow: 1 }}>
        <Outlet />
      </Box>
    </Box>
    </SocketProvider>
  );
}
