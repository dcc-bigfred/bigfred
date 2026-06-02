import type { ReactNode } from "react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  Typography,
} from "@mui/material";
import CloseIcon from "@mui/icons-material/Close";
import { useTranslation } from "react-i18next";

interface ThrottleSetupDialogProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
}

// ThrottleSetupDialog hosts command-station picker, connection status,
// and related alerts moved off the main throttle page.
export default function ThrottleSetupDialog({
  open,
  onClose,
  children,
}: ThrottleSetupDialogProps) {
  const { t } = useTranslation("throttle");

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          pr: 1,
        }}
      >
        <Typography variant="h6" component="span">
          {t("title")}
        </Typography>
        <IconButton
          edge="end"
          onClick={onClose}
          aria-label={t("setup.close")}
        >
          <CloseIcon />
        </IconButton>
      </DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2.5}>{children}</Stack>
      </DialogContent>
    </Dialog>
  );
}
