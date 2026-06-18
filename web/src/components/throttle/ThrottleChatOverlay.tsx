import { useMemo, useState } from "react";
import ReplyIcon from "@mui/icons-material/Reply";
import {
  Box,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useMe } from "../../api/auth";
import { useInterlockings } from "../../api/interlockings";
import {
  driverReplyTargetFromInbound,
  driverChatInterlockingLabel,
  radioMessageOpacity,
  radioMessagesNewestFirst,
  useMyRadio,
  type DriverRadioReplyTarget,
  type RadioMessage,
} from "../../api/radio";
import RadioChatLine from "../radio/RadioChatLine";
import RadioPhrasePickerDialog from "../interlocking/RadioPhrasePickerDialog";
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
  const [reply, setReply] = useState<DriverRadioReplyTarget | null>(null);
  const selfLabel = t("radio:self");
  const now = useRadioMessageClock();

  return (
    <>
      <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
        <DialogTitle>{t("throttle:radio.chat")}</DialogTitle>
        <DialogContent>
          <Box sx={{ maxHeight: 400, overflowY: "auto" }}>
            {messages.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                {t("throttle:radio.chatEmpty")}
              </Typography>
            ) : (
              <Stack spacing={0.5}>
                {messages.map((msg) => (
                  <ChatRow
                    key={msg.messageId}
                    msg={msg}
                    now={now}
                    viewerUserId={me?.id}
                    selfLabel={selfLabel}
                    threadLabel={driverChatInterlockingLabel(msg, interlockingNames)}
                    onReply={() => {
                      const target = driverReplyTargetFromInbound(msg);
                      if (target != null) {
                        setReply(target);
                      }
                    }}
                  />
                ))}
              </Stack>
            )}
          </Box>
        </DialogContent>
      </Dialog>

      {reply && (
        <RadioPhrasePickerDialog
          open
          onClose={() => setReply(null)}
          to={reply.to}
          context={reply.context}
          side="driver"
          targetLabel={reply.targetLabel}
          contextLabel={reply.contextLabel}
        />
      )}
    </>
  );
}

function ChatRow({
  msg,
  now,
  viewerUserId,
  selfLabel,
  threadLabel,
  onReply,
}: {
  msg: RadioMessage;
  now: number;
  viewerUserId?: number;
  selfLabel: string;
  threadLabel: string;
  onReply: () => void;
}) {
  const { t } = useTranslation(["throttle", "radio"]);
  const phraseLabel = t(`radio:phrase.${msg.phrase}`);
  const opacity = radioMessageOpacity(msg.sentAt, now);
  const showReply = driverReplyTargetFromInbound(msg) != null;

  return (
    <Stack
      direction="row"
      alignItems="flex-start"
      spacing={0.5}
      sx={{
        py: 0.5,
        px: 0.5,
        "&:hover": { bgcolor: "action.hover", borderRadius: 1 },
      }}
    >
      <Typography
        variant="body2"
        sx={{ flex: 1, minWidth: 0, wordBreak: "break-word", opacity }}
      >
        <RadioChatLine
          msg={msg}
          phraseLabel={phraseLabel}
          viewerUserId={viewerUserId}
          selfLabel={selfLabel}
          threadLabel={threadLabel}
        />
        {msg.note ? (
          <Typography component="span" variant="body2" color="text.secondary">
            {" "}
            — {msg.note}
          </Typography>
        ) : null}
      </Typography>
      {showReply && (
        <Tooltip title={t("throttle:radio.reply")}>
          <IconButton
            size="small"
            aria-label={t("throttle:radio.reply")}
            onClick={onReply}
            sx={{ width: 36, flexShrink: 0 }}
          >
            <ReplyIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      )}
    </Stack>
  );
}
