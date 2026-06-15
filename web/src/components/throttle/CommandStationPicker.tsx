import {
  FormControl,
  InputLabel,
  MenuItem,
  Select,
} from "@mui/material";
import { useTranslation } from "react-i18next";

interface CommandStationOption {
  id: number;
  name: string;
  kind?: string;
}

interface CommandStationPickerProps {
  stations: CommandStationOption[];
  currentID: number;
  disabled?: boolean;
  allowClear?: boolean;
  onChange: (id: number) => void;
}

// CommandStationPicker is the shared layout command-station dropdown
// used by the throttle setup dialog and the interlocking view.
export default function CommandStationPicker({
  stations,
  currentID,
  disabled,
  allowClear,
  onChange,
}: CommandStationPickerProps) {
  const { t } = useTranslation("throttle");
  if (stations.length === 0) {
    return null;
  }
  return (
    <FormControl fullWidth disabled={disabled}>
      <InputLabel id="command-station-label">{t("commandStation")}</InputLabel>
      <Select
        labelId="command-station-label"
        value={currentID > 0 ? String(currentID) : ""}
        label={t("commandStation")}
        onChange={(ev) => {
          const raw = ev.target.value;
          if (raw === "") {
            onChange(0);
            return;
          }
          const csID = Number(raw);
          if (Number.isFinite(csID) && csID > 0) {
            onChange(csID);
          }
        }}
      >
        {allowClear && (
          <MenuItem value="">
            <em>—</em>
          </MenuItem>
        )}
        {stations.map((s) => (
          <MenuItem key={s.id} value={String(s.id)}>
            {s.kind ? `${s.name} (${s.kind})` : s.name}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
