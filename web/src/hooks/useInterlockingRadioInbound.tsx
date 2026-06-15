import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import {
  formatRadioAlertLine,
  isInboundRadioForInterlocking,
  type RadioMessage,
} from "../api/radio";
import RadioInboundAlert from "../components/radio/RadioInboundAlert";
import { useSocket } from "../context/SocketContext";
import { useRadioInboundSound } from "./useRadioInboundSound";

export interface InterlockingRadioInbound {
  unreadCount: number;
  clearUnread: () => void;
  alertNode: ReactNode;
}

// useInterlockingRadioInbound mirrors throttle inbound radio alerts for
// the signalman group chat (driver → interlocking).
export function useInterlockingRadioInbound(
  interlockingId: number,
  chatVisible: boolean,
): InterlockingRadioInbound {
  const me = useMe().data;
  const { subscribe } = useSocket();
  const { t } = useTranslation("radio");
  const playSound = useRadioInboundSound();
  const [unreadCount, setUnreadCount] = useState(0);
  const [alert, setAlert] = useState<RadioMessage | null>(null);
  const chatVisibleRef = useRef(chatVisible);
  chatVisibleRef.current = chatVisible;

  useEffect(() => {
    if (chatVisible) {
      setUnreadCount(0);
    }
  }, [chatVisible]);

  useEffect(() => {
    return subscribe("radio.message", (payload) => {
      const msg = payload as RadioMessage;
      if (!me || !isInboundRadioForInterlocking(msg, me.id, interlockingId)) {
        return;
      }
      playSound();
      if (!chatVisibleRef.current) {
        setUnreadCount((n) => n + 1);
      }
      setAlert(msg);
    });
  }, [subscribe, me, interlockingId, playSound]);

  const clearUnread = useCallback(() => setUnreadCount(0), []);

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

  return { unreadCount, clearUnread, alertNode };
}
