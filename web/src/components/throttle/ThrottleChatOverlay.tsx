import {
  Box,
  Dialog,
  DialogContent,
  DialogTitle,
  Stack,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import {
  formatRadioChatLine,
  useMyRadio,
  type RadioMessage,
} from "../../api/radio";

interface ThrottleChatOverlayProps {
  onClose: () => void;
}

function formatLine(msg: RadioMessage, phraseLabel: string): string {
  return formatRadioChatLine(msg, phraseLabel);
}

// ThrottleChatOverlay shows the driver's personal radio history.
export default function ThrottleChatOverlay({ onClose }: ThrottleChatOverlayProps) {
  const { t } = useTranslation(["throttle", "radio"]);
  const messages = useMyRadio().data ?? [];

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
                <Typography key={msg.messageId} variant="body2">
                  {formatLine(msg, t(`radio:phrase.${msg.phrase}`))}
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
