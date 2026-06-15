import { Paper, Typography } from "@mui/material";
import { useTranslation } from "react-i18next";

// InterlockingChatPanel is a placeholder until stage 2 (radio comms).
export default function InterlockingChatPanel() {
  const { t } = useTranslation("interlocking");
  return (
    <Paper
      variant="outlined"
      sx={{
        display: "flex",
        flexDirection: "column",
        minHeight: 320,
        maxHeight: "min(70vh, 640px)",
        width: "100%",
        p: 2,
      }}
    >
      <Typography variant="subtitle1" gutterBottom>
        {t("view.chat.title")}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {t("view.chat.placeholder")}
      </Typography>
    </Paper>
  );
}
