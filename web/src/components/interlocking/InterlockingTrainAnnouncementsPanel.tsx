import { useMemo } from "react";
import {
  Box,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Typography,
} from "@mui/material";
import VolumeUpIcon from "@mui/icons-material/VolumeUp";
import { useTranslation } from "react-i18next";

import { trainAnnouncementsFor } from "../../config/trainAnnouncements";
import { useTrainAnnouncementSound } from "../../hooks/useTrainAnnouncementSound";

interface InterlockingTrainAnnouncementsPanelProps {
  interlockingName: string;
}

// InterlockingTrainAnnouncementsPanel lists static PA messages and plays
// them locally on row click (§6.3d).
export default function InterlockingTrainAnnouncementsPanel({
  interlockingName,
}: InterlockingTrainAnnouncementsPanelProps) {
  const { t } = useTranslation(["interlocking", "trainAnnouncements"]);
  const { play, playingSoundKey } = useTrainAnnouncementSound();
  const entries = useMemo(
    () => trainAnnouncementsFor(interlockingName),
    [interlockingName],
  );
  const empty = entries.length === 0;

  return (
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
        <Typography variant="subtitle1">
          {t("interlocking:view.announcements.title")}
        </Typography>
      </Box>
      <Box sx={{ flex: 1, overflowY: "auto" }}>
        {empty ? (
          <Typography variant="body2" color="text.secondary" sx={{ p: 2 }}>
            {t("interlocking:view.announcements.empty")}
          </Typography>
        ) : (
          <List disablePadding>
            {entries.map((entry) => {
              const playing = playingSoundKey === entry.soundKey;
              return (
                <ListItemButton
                  key={entry.soundKey}
                  selected={playing}
                  onClick={() => play(entry.soundKey)}
                >
                  <ListItemText
                    primary={t(`trainAnnouncements:${entry.labelKey}`)}
                  />
                  <VolumeUpIcon
                    fontSize="small"
                    color={playing ? "primary" : "action"}
                  />
                </ListItemButton>
              );
            })}
          </List>
        )}
      </Box>
    </Paper>
  );
}
