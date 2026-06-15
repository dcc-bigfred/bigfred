import { Box } from "@mui/material";

import type { RadioMessage } from "../../api/radio";
import AutoDismissAlert from "../AutoDismissAlert";

interface RadioInboundAlertProps {
  message: RadioMessage | null;
  text: string;
  onDismiss?: () => void;
}

// RadioInboundAlert is the floating toast shown on inbound radio.message.
export default function RadioInboundAlert({
  message,
  text,
  onDismiss,
}: RadioInboundAlertProps) {
  if (message == null) {
    return null;
  }

  return (
    <Box
      sx={{
        position: "fixed",
        top: { xs: 72, sm: 80 },
        left: "50%",
        transform: "translateX(-50%)",
        zIndex: (theme) => theme.zIndex.snackbar,
        width: "min(480px, calc(100vw - 24px))",
        pointerEvents: "auto",
      }}
    >
      <AutoDismissAlert
        severity="info"
        resetKey={message.messageId}
        autoHideMs={6000}
        onClose={onDismiss}
        sx={{ width: "100%", boxShadow: 4 }}
      >
        {text}
      </AutoDismissAlert>
    </Box>
  );
}
