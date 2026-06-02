// Shared palette for the skeuomorphic throttle cockpit (§6.3b).
export const FUNCTION_BUTTON_SIZE_PX = 64;
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
