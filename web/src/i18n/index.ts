// i18next bootstrap. This file owns:
//   1. Static imports of every catalogue JSON (per §7c.5 — tree-shaken
//      into hashed bundle chunks; no runtime translation backend).
//   2. The locale-detection chain (localStorage > navigator > fallback).
//   3. The supported locales + their type, re-exported for components
//      that render the language switcher.
//
// Side-effect note: `i18n.init(...)` runs at module load time, BEFORE
// React boots, so by the time `<I18nextProvider/>` mounts in main.tsx
// the catalogues are already attached. We intentionally do NOT export
// an `init` function — making bootstrapping implicit removes a
// foot-gun where a forgetful component imports `i18n` before init.

import i18n from "i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import { initReactI18next } from "react-i18next";

import plCommon from "./locales/pl/common.json";
import plAuth from "./locales/pl/auth.json";
import plErrors from "./locales/pl/errors.json";
import plRole from "./locales/pl/role.json";
import plHome from "./locales/pl/home.json";
import plLayout from "./locales/pl/layout.json";
import plInterlocking from "./locales/pl/interlocking.json";
import plRadio from "./locales/pl/radio.json";
import plVehicle from "./locales/pl/vehicle.json";
import plUser from "./locales/pl/user.json";
import plSudo from "./locales/pl/sudo.json";
import plThrottle from "./locales/pl/throttle.json";
import plCommandStation from "./locales/pl/commandStation.json";
import plDiagnostics from "./locales/pl/diagnostics.json";
import plFunction from "./locales/pl/function.json";
import plTrainAnnouncements from "./locales/pl/trainAnnouncements.json";
import plAudit from "./locales/pl/audit.json";
import plRentals from "./locales/pl/rentals.json";
import plZ21Remote from "./locales/pl/z21Remote.json";

import enCommon from "./locales/en/common.json";
import enAuth from "./locales/en/auth.json";
import enErrors from "./locales/en/errors.json";
import enRole from "./locales/en/role.json";
import enHome from "./locales/en/home.json";
import enLayout from "./locales/en/layout.json";
import enInterlocking from "./locales/en/interlocking.json";
import enRadio from "./locales/en/radio.json";
import enVehicle from "./locales/en/vehicle.json";
import enUser from "./locales/en/user.json";
import enSudo from "./locales/en/sudo.json";
import enThrottle from "./locales/en/throttle.json";
import enCommandStation from "./locales/en/commandStation.json";
import enDiagnostics from "./locales/en/diagnostics.json";
import enFunction from "./locales/en/function.json";
import enTrainAnnouncements from "./locales/en/trainAnnouncements.json";
import enAudit from "./locales/en/audit.json";
import enRentals from "./locales/en/rentals.json";
import enZ21Remote from "./locales/en/z21Remote.json";

import deCommon from "./locales/de/common.json";
import deAuth from "./locales/de/auth.json";
import deErrors from "./locales/de/errors.json";
import deRole from "./locales/de/role.json";
import deHome from "./locales/de/home.json";
import deLayout from "./locales/de/layout.json";
import deInterlocking from "./locales/de/interlocking.json";
import deRadio from "./locales/de/radio.json";
import deVehicle from "./locales/de/vehicle.json";
import deUser from "./locales/de/user.json";
import deSudo from "./locales/de/sudo.json";
import deThrottle from "./locales/de/throttle.json";
import deCommandStation from "./locales/de/commandStation.json";
import deDiagnostics from "./locales/de/diagnostics.json";
import deFunction from "./locales/de/function.json";
import deTrainAnnouncements from "./locales/de/trainAnnouncements.json";
import deAudit from "./locales/de/audit.json";
import deRentals from "./locales/de/rentals.json";
import deZ21Remote from "./locales/de/z21Remote.json";

// SUPPORTED_LOCALES is the single source of truth. Adding a third
// locale (e.g. "de") is: append it here → mirror every catalogue
// under web/src/i18n/locales/<code>/ → add it to the resources object
// → done. Per §7c.1 we may grow to at most three locales.
export const SUPPORTED_LOCALES = ["pl", "en", "de"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];

// Catalogue map. Keys MUST be Locale values; values MUST contain
// every namespace declared in `ns` below. The shape is used by the
// module augmentation in `types.ts` to provide compile-time key
// safety for `useTranslation` / `t(...)`.
export const resources = {
  pl: {
    common: plCommon,
    auth: plAuth,
    errors: plErrors,
    role: plRole,
    home: plHome,
    layout: plLayout,
    interlocking: plInterlocking,
    radio: plRadio,
    vehicle: plVehicle,
    user: plUser,
    sudo: plSudo,
    throttle: plThrottle,
    commandStation: plCommandStation,
    diagnostics: plDiagnostics,
    function: plFunction,
    trainAnnouncements: plTrainAnnouncements,
    audit: plAudit,
    rentals: plRentals,
    z21Remote: plZ21Remote,
  },
  en: {
    common: enCommon,
    auth: enAuth,
    errors: enErrors,
    role: enRole,
    home: enHome,
    layout: enLayout,
    interlocking: enInterlocking,
    radio: enRadio,
    vehicle: enVehicle,
    user: enUser,
    sudo: enSudo,
    throttle: enThrottle,
    commandStation: enCommandStation,
    diagnostics: enDiagnostics,
    function: enFunction,
    trainAnnouncements: enTrainAnnouncements,
    audit: enAudit,
    rentals: enRentals,
    z21Remote: enZ21Remote,
  },
  de: {
    common: deCommon,
    auth: deAuth,
    errors: deErrors,
    role: deRole,
    home: deHome,
    layout: deLayout,
    interlocking: deInterlocking,
    radio: deRadio,
    vehicle: deVehicle,
    user: deUser,
    sudo: deSudo,
    throttle: deThrottle,
    commandStation: deCommandStation,
    diagnostics: deDiagnostics,
    function: deFunction,
    trainAnnouncements: deTrainAnnouncements,
    audit: deAudit,
    rentals: deRentals,
    z21Remote: deZ21Remote,
  },
} as const;

// LOCALE_STORAGE_KEY is shared between the detector (writes it via
// `caches: ["localStorage"]`) and any imperative code that needs to
// pre-seed the value (e.g. when User.LocalePref lands in M2+).
export const LOCALE_STORAGE_KEY = "bigfred.locale";

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: "pl",
    supportedLngs: SUPPORTED_LOCALES as unknown as string[],
    defaultNS: "common",
    ns: ["common", "auth", "errors", "role", "home", "layout", "interlocking", "radio", "vehicle", "user", "sudo", "throttle", "commandStation", "diagnostics", "function", "trainAnnouncements", "audit", "rentals", "z21Remote"],
    interpolation: {
      // React already escapes everything; double-escaping inside
      // i18next would mangle apostrophes and quotes.
      escapeValue: false,
    },
    detection: {
      // Explicit user choice (localStorage) beats the browser locale,
      // which beats the fallback. Per §7c.8.
      order: ["localStorage", "navigator"],
      lookupLocalStorage: LOCALE_STORAGE_KEY,
      caches: ["localStorage"],
    },
    // returnNull defaults to true in i18next v23+; keep it explicit
    // so a typo-driven undefined doesn't render as the bare key.
    returnNull: false,
  });

// Mirror the active language onto <html lang="…"> so screen readers,
// CSS `:lang()` rules and browser spellcheck pick the right rules.
// Done here (not in a React effect) because the DOM element exists
// long before React mounts, and we want SSR-style correctness from
// the very first paint.
const syncDocumentLang = (lng: string) => {
  if (typeof document !== "undefined") {
    document.documentElement.lang = lng;
  }
};
syncDocumentLang(i18n.resolvedLanguage ?? i18n.language ?? "pl");
i18n.on("languageChanged", syncDocumentLang);

export default i18n;
