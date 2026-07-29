/** Feature flags for this SPA build (hub vs Android phone). */

const android = import.meta.env.VITE_ANDROID === "1";

/**
 * Capabilities derived from the build target. Call sites should check these
 * flags — not whether the build is Android — so phone limits live in one place.
 */
export const capabilities = {
  /** Orange primary AppBar (phone build branding). */
  phoneChrome: android,
  /** Show „Piloty” / Handsets in the Moje / My menu. */
  remotesInMenu: !android,
  /** Editable remotes step in the connection wizard (else greyed out, forced off). */
  remotesConfigurable: !android,
  /** Offer loconet_serial command-station kinds in admin UI. */
  loconetSerial: !android,
} as const;
