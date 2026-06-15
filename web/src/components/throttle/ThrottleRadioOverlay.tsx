import { useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  List,
  ListItemButton,
  ListItemText,
  TextField,
  Typography,
} from "@mui/material";
import { useTranslation } from "react-i18next";

import { useInterlockings } from "../../api/interlockings";
import RadioPhrasePickerDialog from "../interlocking/RadioPhrasePickerDialog";

interface ThrottleRadioOverlayProps {
  layoutId: number;
  vehicleId: number;
  vehicleName: string;
  onClose: () => void;
}

// ThrottleRadioOverlay lets a driver pick an interlocking and send a
// phrase in the context of their current vehicle.
export default function ThrottleRadioOverlay({
  layoutId: _layoutId,
  vehicleId,
  vehicleName,
  onClose,
}: ThrottleRadioOverlayProps) {
  const { t } = useTranslation("throttle");
  const interlockings = useInterlockings().data ?? [];
  const [query, setQuery] = useState("");
  const [pickedId, setPickedId] = useState<number | null>(null);

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return interlockings;
    return interlockings.filter((ilk) => {
      const label = `${ilk.name} ${ilk.location}`.toLowerCase();
      return label.includes(q);
    });
  }, [interlockings, query]);

  const picked = interlockings.find((ilk) => ilk.id === pickedId);

  if (picked) {
    return (
      <RadioPhrasePickerDialog
        open
        onClose={onClose}
        to={{ interlockingId: picked.id }}
        context={{ vehicleId }}
        side="driver"
        targetLabel={picked.name}
        contextLabel={vehicleName}
      />
    );
  }

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>{t("radio.pickInterlocking")}</DialogTitle>
      <DialogContent>
        <TextField
          size="small"
          fullWidth
          autoFocus
          placeholder={t("radio.searchInterlocking")}
          value={query}
          onChange={(ev) => setQuery(ev.target.value)}
          sx={{ mb: 1 }}
        />
        {rows.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            {t("radio.noInterlockings")}
          </Typography>
        ) : (
          <List dense disablePadding>
            {rows.map((ilk) => (
              <ListItemButton key={ilk.id} onClick={() => setPickedId(ilk.id)}>
                <ListItemText
                  primary={ilk.name}
                  secondary={ilk.location || undefined}
                />
              </ListItemButton>
            ))}
          </List>
        )}
      </DialogContent>
    </Dialog>
  );
}
