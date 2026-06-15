import { useMemo, useState } from "react";
import {
  Box,
  Badge,
  IconButton,
  Paper,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import ReplyIcon from "@mui/icons-material/Reply";
import { useTranslation } from "react-i18next";

import { useMe } from "../../api/auth";
import {
  contextLabel,
  radioContextFromMessage,
  radioMessageOpacity,
  radioMessagesNewestFirst,
  useInterlockingRadio,
  type RadioMessage,
  type RadioSendContext,
  type RadioSendTarget,
} from "../../api/radio";
import { useRadioMessageClock } from "../../hooks/useRadioMessageClock";
import RadioChatLine from "../radio/RadioChatLine";
import RadioPhrasePickerDialog from "./RadioPhrasePickerDialog";

interface InterlockingChatPanelProps {
  interlockingId: number;
  unreadCount?: number;
}

interface ReplyTarget {
  to: RadioSendTarget;
  context: RadioSendContext;
  targetLabel: string;
  contextLabel: string;
}

// InterlockingChatPanel shows the signalman group chat with replay,
// live updates and per-line reply actions.
export default function InterlockingChatPanel({
  interlockingId,
  unreadCount = 0,
}: InterlockingChatPanelProps) {
  const { t } = useTranslation(["interlocking", "radio"]);
  const me = useMe().data;
  const { data: rawMessages } = useInterlockingRadio(interlockingId);
  const messages = useMemo(
    () => radioMessagesNewestFirst(rawMessages ?? []),
    [rawMessages],
  );
  const [reply, setReply] = useState<ReplyTarget | null>(null);
  const now = useRadioMessageClock();

  const empty = messages.length === 0;

  return (
    <>
      <Paper
        variant="outlined"
        sx={{
          display: "flex",
          flexDirection: "column",
          minHeight: 320,
          maxHeight: "min(70vh, 640px)",
          width: "100%",
        }}
      >
        <Box sx={{ p: 2, borderBottom: 1, borderColor: "divider" }}>
          <Badge color="error" badgeContent={unreadCount} invisible={unreadCount === 0}>
            <Typography variant="subtitle1">{t("interlocking:view.chat.title")}</Typography>
          </Badge>
        </Box>
        <Box
          sx={{
            flex: 1,
            overflowY: "auto",
            px: 1,
            py: 1,
          }}
        >
          {empty ? (
            <Typography variant="body2" color="text.secondary" sx={{ p: 1 }}>
              {t("interlocking:view.chat.empty")}
            </Typography>
          ) : (
            <Stack spacing={0.5}>
              {messages.map((msg) => (
                <ChatRow
                  key={msg.messageId}
                  msg={msg}
                  now={now}
                  viewerUserId={me?.id}
                  selfLabel={t("radio:self")}
                  showReply={me?.id !== msg.from.userId}
                  onReply={() =>
                    setReply({
                      to: { userId: msg.from.userId },
                      context: radioContextFromMessage(msg),
                      targetLabel: msg.from.login,
                      contextLabel: contextLabel(msg),
                    })
                  }
                />
              ))}
            </Stack>
          )}
        </Box>
      </Paper>

      {reply && (
        <RadioPhrasePickerDialog
          open
          onClose={() => setReply(null)}
          to={reply.to}
          context={reply.context}
          side="signalman"
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
  showReply,
  onReply,
}: {
  msg: RadioMessage;
  now: number;
  viewerUserId?: number;
  selfLabel: string;
  showReply: boolean;
  onReply: () => void;
}) {
  const { t } = useTranslation(["interlocking", "radio"]);
  const phraseLabel = t(`radio:phrase.${msg.phrase}`);
  const opacity = radioMessageOpacity(msg.sentAt, now);

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
        />
        {msg.note ? (
          <Typography component="span" variant="body2" color="text.secondary">
            {" "}
            — {msg.note}
          </Typography>
        ) : null}
      </Typography>
      {showReply && (
        <Tooltip title={t("interlocking:view.chat.reply")}>
          <IconButton
            size="small"
            aria-label={t("interlocking:view.chat.reply")}
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
