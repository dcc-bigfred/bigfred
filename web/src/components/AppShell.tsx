import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AppBar,
  Box,
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
import DashboardIcon from "@mui/icons-material/Dashboard";
import PeopleIcon from "@mui/icons-material/People";
import MapIcon from "@mui/icons-material/Map";
import AccountTreeIcon from "@mui/icons-material/AccountTree";
import DirectionsRailwayIcon from "@mui/icons-material/DirectionsRailway";
import HistoryIcon from "@mui/icons-material/History";
import BugReportIcon from "@mui/icons-material/BugReport";
import PersonIcon from "@mui/icons-material/Person";
import HandshakeIcon from "@mui/icons-material/Handshake";
import TrainIcon from "@mui/icons-material/Train";
import TuneIcon from "@mui/icons-material/Tune";
import VpnKeyIcon from "@mui/icons-material/VpnKey";
import LockResetIcon from "@mui/icons-material/LockReset";
import LogoutIcon from "@mui/icons-material/Logout";
import { Link, Outlet, useMatch, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useLogout, useMe } from "../api/auth";
import { getUserName } from "../utils/getUserName";
import { SocketProvider } from "../context/SocketContext";
import { useSessionExpiryRedirect } from "../hooks/useSessionExpiryRedirect";
import LanguageMenu from "./LanguageMenu";
import { useSudoMobileMenuItems } from "./SudoIndicator";
import MobileNavDrawer, { type MobileNavSection } from "./MobileNavDrawer";
import TopBarMenu, { type TopBarMenuItem } from "./TopBarMenu";
import FullscreenToggleButton from "./throttle/FullscreenToggleButton";

// AppShell renders the top app bar shared by every authenticated
// screen. The post-login pages render inside its <Outlet/>.
//
// Layout, left → right inside the AppBar:
//   [☰] [BigFred] ……spacer…… [layout chip] [⚡][⛶] [menus†] [🌐]
//   † dropdown menus (My / Admin / Account) hide below `md`; the ☰
//   drawer is always available when logged in.
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
  useSessionExpiryRedirect();

  return (
    <SocketProvider enabled={!!me}>
      <AppShellContent />
    </SocketProvider>
  );
}

function AppShellContent() {
  const me = useMe().data;
  const logout = useLogout();
  const navigate = useNavigate();
  const theme = useTheme();
  const isThrottlePage = Boolean(useMatch("/throttle"));
  const isCompactNav = useMediaQuery(theme.breakpoints.down("md"));
  const hideAppTitle = useMediaQuery(theme.breakpoints.down("lg"));
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const onThrottlePage = Boolean(useMatch("/throttle"));

  // Throttle is a fixed-viewport route (AppShell clips to 100dvh). Ensure
  // document scroll is never left locked after leaving — a stale body overflow
  // breaks tables and long pages on every other screen.
  useEffect(() => {
    if (isThrottlePage) {
      return;
    }
    document.body.style.removeProperty("overflow");
    document.documentElement.style.removeProperty("overflow");
  }, [isThrottlePage]);

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
      {
        id: "rentals",
        label: t("nav.my.rentals"),
        icon: <HandshakeIcon fontSize="small" />,
        onClick: () => navigate("/rentals"),
      },
      {
        id: "templates",
        label: t("nav.my.templates"),
        icon: <TuneIcon fontSize="small" />,
        onClick: () => navigate("/vehicle-templates"),
      },
      { id: "divider-audit", divider: true },
      {
        id: "audit-log",
        label: t("nav.administration.auditLog"),
        icon: <HistoryIcon fontSize="small" />,
        onClick: () => navigate("/audit-log"),
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
        onClick: () => navigate("/account/profile"),
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
        onClick: () => navigate("/account/change-pin"),
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

  const closeMobileNav = useCallback(() => setMobileNavOpen(false), []);
  const { items: sudoMobileItems, dialogs: sudoMobileDialogs } =
    useSudoMobileMenuItems();

  const quickNavItems: TopBarMenuItem[] = useMemo(
    () => [
      {
        id: "dashboard",
        label: t("nav.dashboard"),
        icon: <DashboardIcon fontSize="small" />,
        onClick: () => navigate("/"),
      },
      {
        id: "throttle",
        label: t("nav.my.throttle"),
        icon: <SpeedIcon fontSize="small" />,
        onClick: () => navigate("/throttle"),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t],
  );

  const mobileNavSections: MobileNavSection[] = useMemo(() => {
    if (!me) return [];
    const sections: MobileNavSection[] = [
      { id: "quick", items: [...quickNavItems, ...sudoMobileItems] },
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
    quickNavItems,
    sudoMobileItems,
    myItems,
    administrationItems,
    accountItems,
    t,
  ]);

  const accountCaption =
    me &&
    `${getUserName(me)} · ${t(`role:${me.effectiveRole}` as const, {
      defaultValue: me.effectiveRole,
    })}`;

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        ...(isThrottlePage
          ? { height: "100dvh", maxHeight: "100dvh", overflow: "hidden" }
          : { minHeight: "100dvh" }),
      }}
    >
      <AppBar position="sticky">
        <Toolbar
          sx={{
            // Default MUI icons are 24px; small variants 20px — bump both by 4px.
            "& .MuiSvgIcon-root": { fontSize: 28 },
            "& .MuiSvgIcon-fontSizeSmall": { fontSize: 24 },
          }}
        >
          {me && (
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

          <Box sx={{ flexGrow: 1, minWidth: 0 }}>
            {!hideAppTitle && (
              <Typography
                variant="h6"
                component={Link}
                to="/"
                sx={{
                  color: "inherit",
                  textDecoration: "none",
                  "&:hover": { opacity: 0.9 },
                }}
                noWrap
              >
                {t("appName")}
              </Typography>
            )}
          </Box>

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
              <Tooltip title={t("nav.goToDashboard")}>
                <Chip
                  size="small"
                  color="default"
                  component={Link}
                  to="/"
                  clickable
                  icon={<MapIcon fontSize="small" />}
                  label={
                    me.layoutIsSystem
                      ? t("layout:system_default_label")
                      : me.layoutName
                  }
                  sx={{
                    bgcolor: "rgba(255,255,255,0.16)",
                    color: "inherit",
                    textDecoration: "none",
                    "& .MuiChip-icon": { color: "inherit" },
                    "&:hover": { bgcolor: "rgba(255,255,255,0.24)" },
                  }}
                />
              </Tooltip>
            )}

            {me && (
              <>
                <Tooltip title={t("nav.my.throttle")}>
                  <IconButton
                    color="inherit"
                    component={Link}
                    to="/throttle"
                    aria-current={onThrottlePage ? "page" : undefined}
                    aria-label={t("nav.my.throttle")}
                  >
                    <SpeedIcon />
                  </IconButton>
                </Tooltip>
                <FullscreenToggleButton />
              </>
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

      {me && (
        <>
          <MobileNavDrawer
            open={mobileNavOpen}
            onClose={closeMobileNav}
            title={t("nav.drawerTitle")}
            closeLabel={t("nav.closeMenu")}
            sections={mobileNavSections}
            identityLine={accountCaption ?? undefined}
          />
          {sudoMobileDialogs}
        </>
      )}

      <Box
        component="main"
        sx={{
          flexGrow: 1,
          display: "flex",
          flexDirection: "column",
          minHeight: 0,
          minWidth: 0,
        }}
      >
        <Outlet />
      </Box>
    </Box>
  );
}
