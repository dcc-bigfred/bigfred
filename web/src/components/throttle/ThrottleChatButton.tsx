import { Badge, IconButton, Tooltip } from "@mui/material";
import ChatIcon from "@mui/icons-material/Chat";
import { useTranslation } from "react-i18next";

import { cockpit } from "./throttleCockpitTheme";

interface ThrottleChatButtonProps {
  unreadCount: number;
  onOpen: () => void;
}

// ThrottleChatButton opens the driver's radio history overlay and
// shows a red badge for unread inbound messages.
export default function ThrottleChatButton({
  unreadCount,
  onOpen,
}: ThrottleChatButtonProps) {
  const { t } = useTranslation("throttle");
  const hasUnread = unreadCount > 0;

  return (
    <Tooltip title={t("radio.chat")}>
      <IconButton
        size="small"
        onClick={onOpen}
        aria-label={t("radio.chat")}
        sx={{
          color: hasUnread ? "#ff5252" : cockpit.text,
        }}
      >
        <Badge
          color="error"
          variant={hasUnread ? "standard" : "dot"}
          badgeContent={hasUnread ? unreadCount : undefined}
          invisible={!hasUnread}
          sx={{
            "& .MuiBadge-badge": {
              fontWeight: 700,
              minWidth: 18,
              height: 18,
              boxShadow: hasUnread ? "0 0 0 2px rgba(0,0,0,0.35)" : undefined,
            },
          }}
        >
          <ChatIcon fontSize="small" />
        </Badge>
      </IconButton>
    </Tooltip>
  );
}
