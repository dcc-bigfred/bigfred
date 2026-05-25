import { useState } from "react";
import { Badge, IconButton, Tooltip } from "@mui/material";
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

// AdminSudoIndicator drives the AppBar padlock — temporary admin
// elevation that auto-expires after the configured TTL (default
// 2 min). The countdown is rendered as a Badge overlay so the lock
// glyph stays prominent. Click idle → open PIN dialog; click active
// → revoke immediately.
function AdminSudoIndicator() {
  const { active, expiresAt, request, revoke, isPending } = useSudoElevation();
  const remaining = useCountdown(expiresAt);
  const [dialogOpen, setDialogOpen] = useState(false);
  const { t } = useTranslation("sudo");

  const tooltip = active
    ? t("tooltip.admin.active", { remaining: remaining ?? "00:00" })
    : t("tooltip.admin.idle");
  const ariaLabel = active
    ? t("aria.admin.active")
    : t("aria.admin.idle");

  const handleClick = () => {
    if (isPending) return;
    if (active) {
      void revoke();
    } else {
      setDialogOpen(true);
    }
  };

  const handleSubmit = async (pin: string) => {
    await request(pin);
    setDialogOpen(false);
  };

  return (
    <>
      <Tooltip title={tooltip}>
        <span>
          <IconButton
            color={active ? "warning" : "inherit"}
            aria-pressed={active}
            aria-label={ariaLabel}
            onClick={handleClick}
            disabled={isPending}
            size="small"
            sx={{ color: "inherit" }}
          >
            {active && remaining ? (
              <Badge
                badgeContent={remaining}
                color="warning"
                slotProps={{
                  badge: {
                    style: { fontVariantNumeric: "tabular-nums", fontSize: 10 },
                  },
                }}
                overlap="circular"
              >
                <LockOpenIcon fontSize="small" />
              </Badge>
            ) : active ? (
              <LockOpenIcon fontSize="small" />
            ) : (
              <LockIcon fontSize="small" />
            )}
          </IconButton>
        </span>
      </Tooltip>
      <SudoPinDialog
        open={dialogOpen}
        target="admin"
        onCancel={() => setDialogOpen(false)}
        onSubmit={handleSubmit}
      />
    </>
  );
}

// SignalmanIndicator drives the engineer's-cap icon — a permanent
// layout-scoped self-grant of the signalman role. There is no
// countdown; the icon simply reflects the persisted membership.
function SignalmanIndicator() {
  const { active, request, revoke, isPending } = useSignalmanGrant();
  const [dialogOpen, setDialogOpen] = useState(false);
  const { t } = useTranslation("sudo");

  const tooltip = active
    ? t("tooltip.signalman.active")
    : t("tooltip.signalman.idle");
  const ariaLabel = active
    ? t("aria.signalman.active")
    : t("aria.signalman.idle");

  const handleClick = () => {
    if (isPending) return;
    if (active) {
      void revoke();
    } else {
      setDialogOpen(true);
    }
  };

  const handleSubmit = async (pin: string) => {
    await request(pin);
    setDialogOpen(false);
  };

  return (
    <>
      <Tooltip title={tooltip}>
        <span>
          <IconButton
            color={active ? "warning" : "inherit"}
            aria-pressed={active}
            aria-label={ariaLabel}
            onClick={handleClick}
            disabled={isPending}
            size="small"
            sx={{ color: "inherit" }}
          >
            {active ? (
              <EngineeringIcon fontSize="small" />
            ) : (
              <EngineeringOutlinedIcon fontSize="small" />
            )}
          </IconButton>
        </span>
      </Tooltip>
      <SudoPinDialog
        open={dialogOpen}
        target="signalman"
        onCancel={() => setDialogOpen(false)}
        onSubmit={handleSubmit}
      />
    </>
  );
}

// SudoIndicators renders both AppBar entrypoints — the padlock
// (admin sudo) and the engineer's cap (permanent signalman
// self-grant). Mounted once in AppShell.
export default function SudoIndicators() {
  useElevationListener();
  return (
    <>
      <AdminSudoIndicator />
      <SignalmanIndicator />
    </>
  );
}
