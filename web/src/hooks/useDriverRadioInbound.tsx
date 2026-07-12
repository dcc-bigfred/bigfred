import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useMe } from "../api/auth";
import {
  driverReplyTargetFromInbound,
  formatRadioAlertLine,
  isInboundRadioForDriver,
  type DriverRadioReplyTarget,
  type RadioMessage,
} from "../api/radio";
import RadioInboundAlert from "../components/radio/RadioInboundAlert";
import RadioPhrasePickerDialog from "../components/interlocking/RadioPhrasePickerDialog";
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
  const radioChatEnabled = me?.radioChatEnabled ?? true;
  const { subscribe } = useSocket();
  const { t } = useTranslation(["radio", "throttle"]);
  const playSound = useRadioInboundSound();
  const [unreadCount, setUnreadCount] = useState(0);
  const [chatOpen, setChatOpen] = useState(false);
  const [alert, setAlert] = useState<RadioMessage | null>(null);
  const [replyTarget, setReplyTarget] = useState<DriverRadioReplyTarget | null>(null);
  const chatOpenRef = useRef(chatOpen);
  chatOpenRef.current = chatOpen;

  useEffect(() => {
    if (!radioChatEnabled) {
      return;
    }
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
  }, [subscribe, me, playSound, radioChatEnabled]);

  const openChat = useCallback(() => {
    setChatOpen(true);
    setUnreadCount(0);
  }, []);

  const handleReply = useCallback(() => {
    if (alert == null) {
      return;
    }
    const target = driverReplyTargetFromInbound(alert);
    if (target == null) {
      return;
    }
    setAlert(null);
    setReplyTarget(target);
  }, [alert]);

  const replyAvailable =
    alert != null && driverReplyTargetFromInbound(alert) != null;

  const alertNode: ReactNode = radioChatEnabled ? (
    <RadioInboundAlert
      message={alert}
      text={
        alert != null
          ? formatRadioAlertLine(alert, t(`radio:phrase.${alert.phrase}` as const))
          : ""
      }
      onDismiss={() => setAlert(null)}
      replyLabel={t("throttle:radio.reply")}
      onReply={replyAvailable ? handleReply : undefined}
    />
  ) : null;

  const overlay: ReactNode = radioChatEnabled ? (
    <>
      {chatOpen ? <ThrottleChatOverlay onClose={() => setChatOpen(false)} /> : null}
      {replyTarget ? (
        <RadioPhrasePickerDialog
          open
          onClose={() => setReplyTarget(null)}
          to={replyTarget.to}
          context={replyTarget.context}
          side="driver"
          targetLabel={replyTarget.targetLabel}
          contextLabel={replyTarget.contextLabel}
        />
      ) : null}
    </>
  ) : null;

  return { unreadCount: radioChatEnabled ? unreadCount : 0, openChat, overlay, alertNode };
}
