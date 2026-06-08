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
/** Fixed width of the speed / throttle column (always on the right). */
export const THROTTLE_PANEL_WIDTH_PX = 128;

export const cockpit = {
  bg: "#0f1e33",
  bgPanel: "#152a45",
  header: "#0c1829",
  border: "rgba(120, 160, 210, 0.35)",
  borderBright: "rgba(180, 210, 255, 0.5)",
  text: "#e8f0fc",
  textMuted: "rgba(232, 240, 252, 0.65)",
  accent: "#e8b923",
  btnTop: "#3d6a9e",
  btnBottom: "#1e3d66",
  btnActiveTop: "#5a8fd4",
  btnActiveBottom: "#2a5080",
  track: "#0a1220",
  trackHighlight: "rgba(140, 190, 255, 0.35)",
  thumb: "#1a1a1a",
  thumbHighlight: "rgba(255, 255, 255, 0.25)",
  speedGradient:
    "linear-gradient(to top, #1b5e20 0%, #7cb342 25%, #fdd835 55%, #fb8c00 78%, #e53935 100%)",
} as const;

/** Same fill as a function cell on the Throttle page (for editors / pickers). */
export function cockpitFunctionButtonGradient(active = false): string {
  const top = active ? cockpit.btnActiveTop : cockpit.btnTop;
  const bottom = active ? cockpit.btnActiveBottom : cockpit.btnBottom;
  return `linear-gradient(145deg, ${top} 0%, ${bottom} 100%)`;
}
