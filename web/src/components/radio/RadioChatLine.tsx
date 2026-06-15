import { Box } from "@mui/material";

import {
  contextLabel,
  formatRadioMessageTime,
  isRadioSelfMessage,
  radioFromLabel,
  type RadioMessage,
} from "../../api/radio";

interface RadioChatLineProps {
  msg: RadioMessage;
  phraseLabel: string;
  viewerUserId?: number;
  selfLabel: string;
  /** When set, replaces vehicle/train name in the line (e.g. interlocking on throttle). */
  threadLabel?: string;
}

// RadioChatLine renders a single chat history row; non-self senders
// are shown with a bold nickname for quick scanning.
export default function RadioChatLine({
  msg,
  phraseLabel,
  viewerUserId,
  selfLabel,
  threadLabel,
}: RadioChatLineProps) {
  const fromLabel = radioFromLabel(msg, { viewerUserId, selfLabel });
  const isSelf = isRadioSelfMessage(msg, viewerUserId);
  const ctx = threadLabel ?? contextLabel(msg);

  return (
    <>
      {formatRadioMessageTime(msg.sentAt)}{" "}
      (
      {isSelf ? (
        fromLabel
      ) : (
        <Box component="span" sx={{ fontWeight: 700 }}>
          {fromLabel}
        </Box>
      )}
      ) {ctx}: {phraseLabel}
    </>
  );
}
