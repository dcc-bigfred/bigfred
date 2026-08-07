import {
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Stack,
  Typography,
} from "@mui/material";
import { Trans, useTranslation } from "react-i18next";

import type { HelpEntry } from "./helpRegistry";

export default function HelpDialog({
  open,
  onClose,
  entry,
  onDisableRoute,
  onDisableGlobal,
}: {
  open: boolean;
  onClose: () => void;
  entry: HelpEntry;
  pathname: string;
  onDisableRoute: () => void;
  onDisableGlobal: () => void;
}) {
  const { t } = useTranslation(["help", "common"]);

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{t("help:dialog.title")}</DialogTitle>
      <DialogContent>
        <Typography component="div" variant="body1" sx={{ mb: 2 }}>
          <Trans
            ns="help"
            i18nKey={entry.i18nKey}
            components={entry.components ?? {}}
          />
        </Typography>
      </DialogContent>
      <DialogActions
        sx={{
          flexDirection: "column",
          alignItems: "stretch",
          gap: 1,
          px: 3,
          pb: 2,
        }}
      >
        <Stack spacing={0}>
          <FormControlLabel
            control={
              <Checkbox
                onChange={(_, checked) => {
                  if (checked) onDisableRoute();
                }}
              />
            }
            label={t("help:dialog.disableForRoute")}
          />
          <FormControlLabel
            control={
              <Checkbox
                onChange={(_, checked) => {
                  if (checked) onDisableGlobal();
                }}
              />
            }
            label={t("help:dialog.disableGlobal")}
          />
        </Stack>
        <Button onClick={onClose} variant="contained">
          {t("common:actions.close")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
