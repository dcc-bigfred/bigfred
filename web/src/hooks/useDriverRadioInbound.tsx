import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import {
  formatRadioAlertLine,
  isInboundRadioForDriver,
  type RadioMessage,
} from "../api/radio";
import RadioInboundAlert from "../components/radio/RadioInboundAlert";
import ThrottleChatOverlay from "../components/throttle/ThrottleChatOverlay";
import { useSocket } from "../context/SocketContext";
import { useRadioInboundSound } from "./useRadioInboundSound";

export interface DriverRadioInbound {
  unreadCount: number;
  openChat: () => void;
  overlay: ReactNode;
  alertNode: ReactNode;
}

// useDriverRadioInbound listens for signalman → driver radio.message
// pushes on the throttle page: sound, text alert and unread badge.
export function useDriverRadioInbound(): DriverRadioInbound {
  const me = useMe().data;
  const { subscribe } = useSocket();
  const { t } = useTranslation("radio");
  const playSound = useRadioInboundSound();
  const [unreadCount, setUnreadCount] = useState(0);
  const [chatOpen, setChatOpen] = useState(false);
  const [alert, setAlert] = useState<RadioMessage | null>(null);
  const chatOpenRef = useRef(chatOpen);
  chatOpenRef.current = chatOpen;

  useEffect(() => {
    return subscribe("radio.message", (payload) => {
      const msg = payload as RadioMessage;
      if (!me || !isInboundRadioForDriver(msg, me.id)) {
        return;
      }
      playSound();
      if (!chatOpenRef.current) {
        setUnreadCount((n) => n + 1);
      }
      setAlert(msg);
    });
  }, [subscribe, me, playSound]);

  const openChat = useCallback(() => {
    setChatOpen(true);
    setUnreadCount(0);
  }, []);

  const alertNode: ReactNode = (
    <RadioInboundAlert
      message={alert}
      text={
        alert != null
          ? formatRadioAlertLine(alert, t(`phrase.${alert.phrase}` as const))
          : ""
      }
      onDismiss={() => setAlert(null)}
    />
  );

  const overlay: ReactNode = chatOpen ? (
    <ThrottleChatOverlay onClose={() => setChatOpen(false)} />
  ) : null;

  return { unreadCount, openChat, overlay, alertNode };
}
