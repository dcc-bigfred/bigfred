import { useState } from "react";
import TrainIcon from "@mui/icons-material/Train";
import { IconButton, Tooltip } from "@mui/material";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import type { PresenceUser } from "../api/presence";
import { useRequestSudo, useRevokeSignalmanFromUser } from "../api/sudo";
import SudoPinDialog from "./SudoPinDialog";

interface DemoteSignalmanButtonProps {
  layoutId: number;
  user: PresenceUser;
}

export default function DemoteSignalmanButton({
  layoutId,
  user,
}: DemoteSignalmanButtonProps) {
  const me = useMe().data;
  const { t } = useTranslation(["home", "sudo"]);
  const revoke = useRevokeSignalmanFromUser();
  const requestSudo = useRequestSudo();
  const [pinOpen, setPinOpen] = useState(false);

  const isAdmin = me?.effectiveRole === "admin";
  const isSelf = me?.id === user.userId;
  const canDemote = user.role === "signalman" && !isSelf;
  const busy = revoke.isPending || requestSudo.isPending;

  if (!canDemote) {
    return null;
  }

  const demote = async (pin?: string) => {
    if (!isAdmin && pin) {
      await requestSudo.mutateAsync({ layoutId, pin });
    }
    await revoke.mutateAsync({ layoutId, userId: user.userId });
    setPinOpen(false);
  };

  const handleClick = () => {
    if (isAdmin) {
      void demote();
      return;
    }
    setPinOpen(true);
  };

  return (
    <>
      <Tooltip title={t("home:onlineUsers.demoteSignalman", { login: user.login })}>
        <span>
          <IconButton
            size="small"
            onClick={handleClick}
            disabled={busy}
            aria-label={t("home:onlineUsers.demoteSignalman", { login: user.login })}
          >
            <TrainIcon fontSize="small" />
          </IconButton>
        </span>
      </Tooltip>

      <SudoPinDialog
        open={pinOpen}
        target="demoteSignalman"
        targetLogin={user.login}
        onCancel={() => setPinOpen(false)}
        onSubmit={(pin) => demote(pin)}
      />
    </>
  );
}
