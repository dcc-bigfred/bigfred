import { useState } from "react";
import EngineeringIcon from "@mui/icons-material/Engineering";
import { IconButton, Tooltip } from "@mui/material";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import type { PresenceUser } from "../api/presence";
import { useGrantSignalmanToUser, useRequestSudo } from "../api/sudo";
import SudoPinDialog from "./SudoPinDialog";

interface PromoteSignalmanButtonProps {
  layoutId: number;
  user: PresenceUser;
}

export default function PromoteSignalmanButton({
  layoutId,
  user,
}: PromoteSignalmanButtonProps) {
  const me = useMe().data;
  const { t } = useTranslation(["home", "sudo"]);
  const grant = useGrantSignalmanToUser();
  const requestSudo = useRequestSudo();
  const [pinOpen, setPinOpen] = useState(false);

  const isAdmin = me?.effectiveRole === "admin";
  const isSelf = me?.id === user.userId;
  const canPromote = user.role === "driver" && !isSelf;
  const busy = grant.isPending || requestSudo.isPending;

  if (!canPromote) {
    return null;
  }

  const promote = async (pin?: string) => {
    if (!isAdmin && pin) {
      await requestSudo.mutateAsync({ layoutId, pin });
    }
    await grant.mutateAsync({ layoutId, userId: user.userId });
    setPinOpen(false);
  };

  const handleClick = () => {
    if (isAdmin) {
      void promote();
      return;
    }
    setPinOpen(true);
  };

  return (
    <>
      <Tooltip title={t("home:onlineUsers.promoteSignalman", { login: user.login })}>
        <span>
          <IconButton
            size="small"
            onClick={handleClick}
            disabled={busy}
            aria-label={t("home:onlineUsers.promoteSignalman", { login: user.login })}
          >
            <EngineeringIcon fontSize="small" />
          </IconButton>
        </span>
      </Tooltip>

      <SudoPinDialog
        open={pinOpen}
        target="promoteSignalman"
        targetLogin={user.login}
        onCancel={() => setPinOpen(false)}
        onSubmit={(pin) => promote(pin)}
      />
    </>
  );
}
