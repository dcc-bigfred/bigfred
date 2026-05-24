import { useState } from "react";
import {
  Box,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Tooltip,
} from "@mui/material";
import LanguageIcon from "@mui/icons-material/Language";
import CheckIcon from "@mui/icons-material/Check";
import { useTranslation } from "react-i18next";

import {
  SUPPORTED_LOCALES,
  type Locale,
} from "../i18n";

// LanguageMenu renders the icon-button + dropdown that lets the user
// switch the active locale at runtime. Per §7c.8 it lives inside
// AppShell, next to the (future) user menu.
//
// Implementation notes:
//   * `i18n.changeLanguage(code)` re-renders every component that
//     uses `useTranslation()`, and persists the choice through the
//     LanguageDetector's `caches: ["localStorage"]` config.
//   * The current locale is highlighted with a CheckIcon so the
//     menu doubles as a status indicator.
//   * Locale display labels live in `common.language.options.<code>`
//     and are themselves translated, so a Polish user sees "Polski /
//     Angielski" while an English user sees "Polish / English".
export default function LanguageMenu() {
  const { t, i18n } = useTranslation("common");
  const [anchor, setAnchor] = useState<null | HTMLElement>(null);

  const active = (i18n.resolvedLanguage ?? i18n.language ?? "pl") as Locale;

  const handleChoose = (loc: Locale) => {
    setAnchor(null);
    if (loc === active) return;
    void i18n.changeLanguage(loc);
  };

  return (
    <Box>
      <Tooltip title={t("language.menuLabel")}>
        <IconButton
          color="inherit"
          aria-label={t("language.menuLabel")}
          aria-haspopup="menu"
          aria-controls={anchor ? "language-menu" : undefined}
          aria-expanded={anchor ? "true" : undefined}
          onClick={(e) => setAnchor(e.currentTarget)}
        >
          <LanguageIcon />
        </IconButton>
      </Tooltip>

      <Menu
        id="language-menu"
        anchorEl={anchor}
        open={Boolean(anchor)}
        onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
      >
        {SUPPORTED_LOCALES.map((loc) => {
          const isActive = loc === active;
          return (
            <MenuItem
              key={loc}
              onClick={() => handleChoose(loc)}
              selected={isActive}
            >
              <ListItemIcon>{isActive ? <CheckIcon fontSize="small" /> : null}</ListItemIcon>
              <ListItemText
                // Each locale key (`pl`, `en`, …) maps to a localised
                // label inside common.language.options.
                primary={t(`language.options.${loc}` as const)}
              />
            </MenuItem>
          );
        })}
      </Menu>
    </Box>
  );
}
