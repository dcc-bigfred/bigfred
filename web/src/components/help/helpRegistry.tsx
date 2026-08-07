import type { ReactElement } from "react";
import PlaylistAddIcon from "@mui/icons-material/PlaylistAdd";
import SpeedIcon from "@mui/icons-material/Speed";
import TuneIcon from "@mui/icons-material/Tune";

export type HelpI18nKey =
  | "dashboard"
  | "myVehicles"
  | "rentals"
  | "myTrains"
  | "helpMessage";

export type HelpNamespace = "help" | "dccPool";

export type HelpEntry = {
  /** i18n namespace; defaults to "help". */
  ns?: HelpNamespace;
  i18nKey: HelpI18nKey;
  components?: Record<string, ReactElement>;
};

const iconSx = { fontSize: 18, verticalAlign: "middle", mx: 0.25 } as const;

export const HELP_REGISTRY: Record<string, HelpEntry> = {
  "/": {
    i18nKey: "dashboard",
    components: {
      addIcon: <PlaylistAddIcon sx={iconSx} color="action" />,
      throttleIcon: <SpeedIcon sx={iconSx} color="action" />,
    },
  },
  "/my/vehicles": {
    i18nKey: "myVehicles",
    components: {
      addIcon: <PlaylistAddIcon sx={iconSx} color="action" />,
      functionsIcon: <TuneIcon sx={iconSx} color="action" />,
    },
  },
  "/rentals": {
    i18nKey: "rentals",
  },
  "/my/trains": {
    i18nKey: "myTrains",
  },
  "/dcc-pools": {
    ns: "dccPool",
    i18nKey: "helpMessage",
  },
};

export function getHelpEntry(pathname: string): HelpEntry | null {
  return HELP_REGISTRY[pathname] ?? null;
}
