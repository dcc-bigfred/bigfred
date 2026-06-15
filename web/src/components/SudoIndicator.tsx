import { useCallback, useMemo, useState, type ReactNode } from "react";
import LockIcon from "@mui/icons-material/Lock";
import LockOpenIcon from "@mui/icons-material/LockOpen";
import EngineeringIcon from "@mui/icons-material/Engineering";
import EngineeringOutlinedIcon from "@mui/icons-material/EngineeringOutlined";
import { useTranslation } from "react-i18next";

import {
  useElevationListener,
  useSignalmanGrant,
  useSudoElevation,
} from "../hooks/useElevation";
import { useCountdown } from "../hooks/useCountdown";
import SudoPinDialog from "./SudoPinDialog";
import { type TopBarMenuItem } from "./TopBarMenu";

function useAdminSudoControl() {
  const { active, expiresAt, request, revoke, isPending } = useSudoElevation();
  const remaining = useCountdown(expiresAt);
  const [dialogOpen, setDialogOpen] = useState(false);

  const activate = useCallback(() => {
    if (isPending) return;
    if (active) {
      void revoke();
    } else {
      setDialogOpen(true);
    }
  }, [active, isPending, revoke]);

  const handleSubmit = useCallback(
    async (pin: string) => {
      await request(pin);
      setDialogOpen(false);
    },
    [request],
  );

  return {
    active,
    remaining,
    isPending,
    dialogOpen,
    setDialogOpen,
    activate,
    handleSubmit,
  };
}

function useSignalmanSudoControl() {
  const { active, request, revoke, isPending } = useSignalmanGrant();
  const [dialogOpen, setDialogOpen] = useState(false);

  const activate = useCallback(() => {
    if (isPending) return;
    if (active) {
      void revoke();
    } else {
      setDialogOpen(true);
    }
  }, [active, isPending, revoke]);

  const handleSubmit = useCallback(
    async (pin: string) => {
      await request(pin);
      setDialogOpen(false);
    },
    [request],
  );

  return {
    active,
    isPending,
    dialogOpen,
    setDialogOpen,
    activate,
    handleSubmit,
  };
}

/** Sudo elevation entries for the mobile nav drawer. */
export function useSudoMobileMenuItems(): {
  items: TopBarMenuItem[];
  dialogs: ReactNode;
} {
  useElevationListener();
  const { t } = useTranslation("sudo");
  const admin = useAdminSudoControl();
  const signalman = useSignalmanSudoControl();

  const items = useMemo((): TopBarMenuItem[] => {
    const adminTooltip = admin.active
      ? t("tooltip.admin.active", { remaining: admin.remaining ?? "00:00" })
      : undefined;
    const signalmanTooltip = signalman.active
      ? t("tooltip.signalman.active")
      : undefined;

    return [
      {
        id: "sudo-admin",
        label: admin.active ? t("nav.revokeAdmin") : t("nav.elevateAdmin"),
        icon: admin.active ? (
          <LockOpenIcon fontSize="small" color="warning" />
        ) : (
          <LockIcon fontSize="small" />
        ),
        onClick: admin.activate,
        disabled: admin.isPending,
        tooltip: adminTooltip,
      },
      {
        id: "sudo-signalman",
        label: signalman.active
          ? t("nav.revokeSignalman")
          : t("nav.becomeSignalman"),
        icon: signalman.active ? (
          <EngineeringIcon fontSize="small" color="warning" />
        ) : (
          <EngineeringOutlinedIcon fontSize="small" />
        ),
        onClick: signalman.activate,
        disabled: signalman.isPending,
        tooltip: signalmanTooltip,
      },
    ];
  }, [
    admin.active,
    admin.remaining,
    admin.isPending,
    admin.activate,
    signalman.active,
    signalman.isPending,
    signalman.activate,
    t,
  ]);

  const dialogs = (
    <>
      <SudoPinDialog
        open={admin.dialogOpen}
        target="admin"
        onCancel={() => admin.setDialogOpen(false)}
        onSubmit={admin.handleSubmit}
      />
      <SudoPinDialog
        open={signalman.dialogOpen}
        target="signalman"
        onCancel={() => signalman.setDialogOpen(false)}
        onSubmit={signalman.handleSubmit}
      />
    </>
  );

  return { items, dialogs };
}
