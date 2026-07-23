// Shared palette for the skeuomorphic throttle cockpit (§6.3b).
export const FUNCTION_BUTTON_SIZE_PX = 90;
/** Icon size inside a function grid cell (Throttle cockpit). */
export const COCKPIT_FUNCTION_ICON_PX = 70;
/** Distance from the top inner edge of the cell to the icon (Throttle cockpit). */
export const FUNCTION_BUTTON_ICON_TOP_PX = 4;
/** Function name label under the icon (Throttle cockpit). */
export const FUNCTION_BUTTON_LABEL_FONT_SIZE_PX = 11;
/** F-number badge in the top-left corner of the cell (Throttle cockpit). */
export const FUNCTION_BUTTON_NUM_FONT_SIZE_PX = 10;
/** Inset of the F-number badge from the top and left inner edges. */
export const FUNCTION_BUTTON_NUM_INSET_PX = 4;
/** Horizontal and vertical gap between function grid cells (Throttle cockpit). */
export const FUNCTION_BUTTON_GRID_GAP_PX = 6;
/** Icon size inside a function list row (Throttle cockpit). */
export const FUNCTION_LIST_ICON_PX = 28;
/** Vertical gap between function list rows (Throttle cockpit). */
export const FUNCTION_LIST_ROW_GAP_PX = 4;
/** Fixed width of the speed / throttle column (always on the right). */
export const THROTTLE_PANEL_WIDTH_PX = 128;

export const cockpit = {
  bg: "#0f1e33",
  bgPanel: "#152a45",
  header: "#0c1829",
  border: "rgba(120, 160, 210, 0.35)",
  borderBright: "rgba(180, 210, 255, 0.5)",
  /** Opaque borders for function cells — avoid rgba fringe on old WebViews. */
  fnBorder: "#4a6a8a",
  fnBorderActive: "#7aa3cc",
  fnBorderDisabled: "#2a3d55",
  text: "#e8f0fc",
  textMuted: "rgba(232, 240, 252, 0.65)",
  /** Opaque muted text for disabled function labels (no opacity compositing). */
  textDisabled: "#6a7d96",
  accent: "#e8b923",
  btnTop: "#3d6a9e",
  btnBottom: "#1e3d66",
  btnActiveTop: "#5a8fd4",
  btnActiveBottom: "#2a5080",
  /** Solid fills for function cells (flat; no gradient compositing). */
  fnFill: "#1e3d66",
  fnFillActive: "#3d6a9e",
  fnFillHover: "#2a5080",
  fnFillActiveHover: "#4a7ab0",
  fnFillDisabled: "#152a45",
  track: "#0a1220",
  trackHighlight: "rgba(140, 190, 255, 0.35)",
  thumb: "#1a1a1a",
  thumbHighlight: "rgba(255, 255, 255, 0.25)",
  speedGradient:
    "linear-gradient(to top, #1b5e20 0%, #7cb342 25%, #fdd835 55%, #fb8c00 78%, #e53935 100%)",
} as const;

/** Solid fill matching Throttle function cells (editors / pickers / buttons). */
export function cockpitFunctionButtonFill(active = false): string {
  return active ? cockpit.fnFillActive : cockpit.fnFill;
}

/**
 * Thin cockpit-coloured scrollbar for the functions panel (list + tiles share one scroller).
 * Opaque hex only — avoid rgba / inset shadows on old Android WebViews.
 */
export const cockpitScrollbarSx = {
  scrollbarWidth: "thin" as const,
  scrollbarColor: `${cockpit.fnBorder} ${cockpit.track}`,
  "&::-webkit-scrollbar": {
    width: 8,
    height: 8,
  },
  "&::-webkit-scrollbar-track": {
    backgroundColor: cockpit.track,
  },
  "&::-webkit-scrollbar-thumb": {
    backgroundColor: cockpit.fnBorder,
    borderRadius: 4,
    border: `2px solid ${cockpit.track}`,
    backgroundClip: "padding-box",
  },
  "&::-webkit-scrollbar-thumb:hover": {
    backgroundColor: cockpit.fnBorderActive,
  },
  "&::-webkit-scrollbar-corner": {
    backgroundColor: cockpit.track,
  },
} as const;

/**
 * Flat surface styles for function tiles/rows.
 * Solid fill + opaque border; no gradient / box-shadow (old Android WebView speckles).
 */
export function cockpitFunctionSurface(
  active = false,
  disabled = false,
): { bgcolor: string; border: string } {
  if (disabled) {
    return {
      bgcolor: cockpit.fnFillDisabled,
      border: `1px solid ${cockpit.fnBorderDisabled}`,
    };
  }
  return {
    bgcolor: cockpitFunctionButtonFill(active),
    border: `1px solid ${active ? cockpit.fnBorderActive : cockpit.fnBorder}`,
  };
}

/** @deprecated Use cockpitFunctionButtonFill — kept for any remaining call sites. */
export function cockpitFunctionButtonGradient(active = false): string {
  return cockpitFunctionButtonFill(active);
}
