import {
  Box,
  Dialog,
  DialogContent,
  DialogTitle,
  Stack,
  Typography,
} from "@mui/material";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";

import { useMe } from "../../api/auth";
import { useInterlockings } from "../../api/interlockings";
import {
  driverChatInterlockingLabel,
  radioMessageOpacity,
  radioMessagesNewestFirst,
  useMyRadio,
} from "../../api/radio";
import RadioChatLine from "../radio/RadioChatLine";
import { useRadioMessageClock } from "../../hooks/useRadioMessageClock";

interface ThrottleChatOverlayProps {
  onClose: () => void;
}

// ThrottleChatOverlay shows the driver's personal radio history.
export default function ThrottleChatOverlay({ onClose }: ThrottleChatOverlayProps) {
  const { t } = useTranslation(["throttle", "radio"]);
  const me = useMe().data;
  const interlockings = useInterlockings().data ?? [];
  const interlockingNames = useMemo(
    () => new Map(interlockings.map((ilk) => [ilk.id, ilk.name])),
    [interlockings],
  );
  const { data: rawMessages } = useMyRadio();
  const messages = useMemo(
    () => radioMessagesNewestFirst(rawMessages ?? []),
    [rawMessages],
  );
  const selfLabel = t("radio:self");
  const now = useRadioMessageClock();

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t("throttle:radio.chat")}</DialogTitle>
      <DialogContent>
        <Box sx={{ maxHeight: 400, overflowY: "auto" }}>
          {messages.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              {t("throttle:radio.chatEmpty")}
            </Typography>
          ) : (
            <Stack spacing={1}>
              {messages.map((msg) => (
                <Typography
                  key={msg.messageId}
                  variant="body2"
                  sx={{ opacity: radioMessageOpacity(msg.sentAt, now) }}
                >
                  <RadioChatLine
                    msg={msg}
                    phraseLabel={t(`radio:phrase.${msg.phrase}`)}
                    viewerUserId={me?.id}
                    selfLabel={selfLabel}
                    threadLabel={driverChatInterlockingLabel(msg, interlockingNames)}
                  />
                  {msg.note ? (
                    <Typography component="span" variant="body2" color="text.secondary">
                      {" "}
                      — {msg.note}
                    </Typography>
                  ) : null}
                </Typography>
              ))}
            </Stack>
          )}
        </Box>
      </DialogContent>
    </Dialog>
  );
}
