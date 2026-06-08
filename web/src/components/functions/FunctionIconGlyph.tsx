import type { CSSProperties, ReactNode } from "react";

import {
  functionIconAssetUrl,
  hasFunctionIconAsset,
} from "../../icons/functionIconAssets";
import { cockpit } from "../throttle/throttleCockpitTheme";
import AssetFunctionIcon from "./AssetFunctionIcon";
import ArtworkFunctionGlyph, {
  hasArtworkFunctionGlyph,
} from "./ArtworkFunctionGlyph";
import UnknownFunctionGlyph from "./UnknownFunctionGlyph";

/** Slugs from GET /api/v1/function-icons (§3a.8). */
export type FunctionIconSlug = string;

const STROKE = 1.75;

function Svg({
  size,
  children,
  style,
  className,
}: {
  size: number;
  children: ReactNode;
  style?: CSSProperties;
  className?: string;
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      style={style}
      className={className}
      aria-hidden
    >
      <g
        stroke="currentColor"
        strokeWidth={STROKE}
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        {children}
      </g>
    </svg>
  );
}

// --- glyph primitives (industrial line pictograms, cockpit-friendly) ---

const rays = (
  <>
    <path d="M12 2v2" />
    <path d="M12 20v2" />
    <path d="M4.2 4.2l1.4 1.4" />
    <path d="M18.4 18.4l1.4 1.4" />
    <path d="M2 12h2" />
    <path d="M20 12h2" />
    <path d="M4.2 19.8l1.4-1.4" />
    <path d="M18.4 5.6l1.4-1.4" />
  </>
);

const bulb = (
  <path d="M9 15a3 3 0 006 0 5.5 5.5 0 00-6 0zm3 4v2" />
);

/** Bogie / truck (inspection from below). */
const locoBogie = (
  <>
    <path d="M4 16h16" />
    <path d="M5.5 16V10.5h13V16" />
    <circle cx="8.25" cy="18" r="1.75" />
    <circle cx="15.75" cy="18" r="1.75" />
    <path d="M8.25 16.25h7.5" />
    <path d="M12 10.5V8" />
    <circle cx="12" cy="6.75" r="1.2" fill="currentColor" stroke="none" />
  </>
);

const shuntingStairs = (
  <path d="M4 18V7M7.5 18V11M11 18V14M14.5 18V9" />
);

const shuntingBulb = (
  <>
    <path d="M17.5 14a2.1 2.1 0 10-4.2 0 3.8 3.8 0 004.2 0z" />
    <path d="M15.4 16.8v1.7" />
    <path d="M19.2 9.2l.7.7M20.5 11.5h1M19.2 13.8l.7-.7" />
  </>
);

const GLYPHS: Record<string, ReactNode> = {
  light: (
    <>
      {rays}
      {bulb}
    </>
  ),
  engine: (
    <>
      <path d="M5 16V11l2.5-3.5h7L17 11v5H5z" />
      <path d="M12 7.5V4h3v3.5H12z" />
      <circle cx="8.5" cy="17" r="2" />
      <circle cx="15.5" cy="17" r="2" />
      <path d="M8.5 15h7" />
    </>
  ),
  sound: (
    <>
      <path d="M8 9v6h3l4 4V5l-4 4H8z" />
      <path d="M17 9a3 3 0 010 6" />
      <path d="M18.5 7a5.5 5.5 0 010 10" />
    </>
  ),
  coupler: (
    <>
      <path d="M6 12h5" />
      <path d="M13 12h5" />
      <circle cx="11.5" cy="12" r="2.5" />
      <path d="M9 9l-2-2M9 15l-2 2M15 9l2-2M15 15l2 2" />
    </>
  ),
  interior_light: (
    <>
      <rect x="5" y="8" width="14" height="8" rx="1" />
      {rays}
      <path d="M12 11v2" />
    </>
  ),
  engine_room_light: (
    <>
      <rect x="4" y="6" width="16" height="12" rx="1" />
      <path d="M8 10h8M8 14h5" />
      <circle cx="17" cy="14" r="1" fill="currentColor" stroke="none" />
    </>
  ),
  shunting_steps_light: (
    <>
      {shuntingStairs}
      {shuntingBulb}
    </>
  ),
  inspection_light: <>{locoBogie}</>,
  vestibule_lights: (
    <>
      <path d="M5 8h14v8H5z" />
      <path d="M9 12h6" />
      {rays}
    </>
  ),
  destination_board_lights: (
    <>
      <rect x="5" y="9" width="14" height="6" rx="1" />
      <path d="M8 12h8" />
      {rays}
    </>
  ),
  door: (
    <>
      <path d="M7 5h10v14H7z" />
      <path d="M15 12h.01" />
      <path d="M7 5l5-2 5 2" />
    </>
  ),
  smoke: (
    <>
      <path d="M8 16c0-2 1.5-3 3-3s3 1 3 3" />
      <path d="M6 13c0-1.5 1-2.5 2.5-2.5S11 11.5 11 13" />
      <path d="M13 11c0-1 1-1.5 2-1.5s2 .5 2 1.5" />
    </>
  ),
  speaker: (
    <>
      <path d="M7 9v6h2l4 4V5l-4 4H7z" />
      <path d="M16 10.5a2.5 2.5 0 010 3" />
    </>
  ),
  toilet: (
    <>
      <path d="M8 6h8v3a4 4 0 01-4 4H10a4 4 0 01-4-4V6z" />
      <path d="M10 17h4v2H10z" />
    </>
  ),
  compressor: (
    <>
      <rect x="5" y="8" width="14" height="8" rx="2" />
      <path d="M9 12h6M5 12H3M21 12h-2" />
    </>
  ),
  brake_sound: (
    <>
      <circle cx="12" cy="12" r="5" />
      <path d="M8 9v6h3l4 4V5l-4 4H8z" opacity="0.9" />
    </>
  ),
  coal_shoveling: (
    <>
      <path d="M6 18l6-10 6 10" />
      <path d="M8 14h8" />
      <path d="M5 18h14" />
    </>
  ),
  fan: (
    <>
      <circle cx="12" cy="12" r="2" />
      <path d="M12 4v4M12 16v4M4 12h4M16 12h4M6.3 6.3l2.8 2.8M14.9 14.9l2.8 2.8M6.3 17.7l2.8-2.8M14.9 7.1l2.8-2.8" />
    </>
  ),
  hand_brake: (
    <>
      <path d="M12 5v14" />
      <path d="M8 9h8" />
      <circle cx="12" cy="16" r="2" />
    </>
  ),
  injector: (
    <>
      <path d="M6 18V8l4-3 4 3v10" />
      <path d="M8 14h8" />
      <path d="M10 11h4" />
    </>
  ),
  mute_sounds: (
    <>
      <path d="M8 9v6h3l4 4V5l-4 4H8z" />
      <path d="M17 9l4 6M21 9l-4 6" />
    </>
  ),
  radio_command: (
    <>
      <path d="M5 16a7 7 0 0114 0" />
      <path d="M8.5 16a3.5 3.5 0 017 0" />
      <path d="M12 12v4" />
      <circle cx="12" cy="18" r="1" fill="currentColor" stroke="none" />
    </>
  ),
  valve: (
    <>
      <path d="M6 12h12" />
      <circle cx="12" cy="12" r="3" />
      <path d="M12 5v3M12 16v3" />
    </>
  ),
  wheels: (
    <>
      <circle cx="8" cy="14" r="3" />
      <circle cx="16" cy="14" r="3" />
      <path d="M5 14h14M8 11V8M16 11V8" />
    </>
  ),
  wipers: (
    <>
      <path d="M4 16s4-6 8-6 8 6 8 6" />
      <path d="M12 10V6" />
      <circle cx="12" cy="5" r="1" fill="currentColor" stroke="none" />
    </>
  ),
  sander: (
    <>
      <path d="M6 18l4-8 4 8" />
      <path d="M8 15h8" />
      <path d="M10 12h4" />
    </>
  ),
  pantograph: (
    <>
      <path d="M12 4v8" />
      <path d="M8 12h8" />
      <path d="M6 20h12" />
      <path d="M10 12l2-4 2 4" />
    </>
  ),
  volume_up: (
    <>
      <path d="M8 10v4h2l4 4V6l-4 4H8z" />
      <path d="M17 9l3 3-3 3M20 8l3 3-3 3" />
    </>
  ),
  volume_down: (
    <>
      <path d="M8 10v4h2l4 4V6l-4 4H8z" />
      <path d="M18 12h4" />
    </>
  ),
  heavy_load: (
    <>
      <path d="M6 10h12v8H6z" />
      <path d="M8 10V7h8v3" />
      <path d="M9 14h2M13 14h2" />
    </>
  ),
  wifi: (
    <>
      <path d="M5 16a11 11 0 0114 0" />
      <path d="M8 13.5a5.5 5.5 0 018 0" />
      <path d="M11 11.5a2 2 0 012 0" />
      <circle cx="12" cy="17" r="1" fill="currentColor" stroke="none" />
    </>
  ),
  pc2_signal: (
    <>
      <rect x="5" y="6" width="14" height="12" rx="1" />
      <path d="M8 10h8M8 14h4" />
      <path d="M16 14h.01" />
    </>
  ),
  coupling: (
    <>
      <path d="M5 12h6" />
      <circle cx="13" cy="12" r="2" />
      <path d="M17 12h2" />
      <path d="M19 10v4" />
    </>
  ),
  uncoupling: (
    <>
      <path d="M5 12h5" />
      <path d="M14 12h5" />
      <path d="M11 9l2 3-2 3" />
    </>
  ),
  oil_pump: (
    <>
      <path d="M8 18V9l4-2 4 2v9" />
      <path d="M10 14h4" />
      <path d="M6 18h12" />
    </>
  ),
  brake_sound_mute: (
    <>
      <circle cx="12" cy="12" r="5" />
      <path d="M9 10l6 6M15 10l-6 6" />
    </>
  ),
  wheel_squeal: (
    <>
      <circle cx="12" cy="14" r="3" />
      <path d="M7 8l2 2M15 8l-2 2M5 6l1 1M19 6l-1 1" />
    </>
  ),
  bell: (
    <>
      <path d="M8 16h8" />
      <path d="M12 6c-2 0-3 1.5-3 3.5v3.5h6V9.5C15 7.5 14 6 12 6z" />
      <path d="M10 18h4" />
    </>
  ),
  coal_bunker: (
    <>
      <path d="M5 16h14l-2-8H7l-2 8z" />
      <path d="M8 12h8" />
    </>
  ),
  watering: (
    <>
      <path d="M12 4c-2 2-4 4-4 7a4 4 0 008 0c0-3-2-5-4-7z" />
      <path d="M9 18h6" />
    </>
  ),
  crane_up: (
    <>
      <path d="M12 19V8" />
      <path d="M8 12h8" />
      <path d="M12 5l-3 3h6l-3-3z" />
      <path d="M5 19h14" />
    </>
  ),
  crane_down: (
    <>
      <path d="M12 5v11" />
      <path d="M8 12h8" />
      <path d="M12 19l-3-3h6l-3 3z" />
      <path d="M5 19h14" />
    </>
  ),
  crane_left: (
    <>
      <path d="M19 12H8" />
      <path d="M12 8v8" />
      <path d="M5 12l3-3v6l-3-3z" />
    </>
  ),
  crane_right: (
    <>
      <path d="M5 12h11" />
      <path d="M12 8v8" />
      <path d="M19 12l-3-3v6l3-3z" />
    </>
  ),
  crane_hook: (
    <>
      <path d="M12 5v6" />
      <path d="M9 11h6" />
      <path d="M12 17c0 1.5-1 2.5-2 2.5S8 18.5 8 17" />
    </>
  ),
  sifa: (
    <>
      <path d="M12 5l7 14H5l7-14z" />
      <path d="M12 10v4" />
      <path d="M12 16h.01" />
    </>
  ),
  firebox: (
    <>
      <rect x="6" y="8" width="12" height="10" rx="1" />
      <path d="M9 14c1-2 2-3 3-3s2 1 3 3" />
    </>
  ),
  steam_release: (
    <>
      <path d="M8 16c0-2 1.5-3 3-3s3 1 3 3" />
      <path d="M12 8v5" />
      <path d="M10 6h4" />
    </>
  ),
  window: (
    <>
      <rect x="5" y="6" width="14" height="12" rx="1" />
      <path d="M5 12h14M12 6v12" />
    </>
  ),
  buffer: (
    <>
      <path d="M6 14h12" />
      <path d="M8 14V9h8v5" />
      <path d="M10 9V6h4v3" />
    </>
  ),
  danger: (
    <>
      <path d="M12 5l7 14H5l7-14z" />
      <path d="M12 10v4" />
      <path d="M12 16h.01" />
    </>
  ),
  engineer_laugh: (
    <>
      <circle cx="12" cy="10" r="4" />
      <path d="M8 17c1 2 2.5 3 4 3s3-1 4-3" />
      <path d="M9 9h.01M15 9h.01" />
    </>
  ),
  stairs: (
    <>
      <path d="M6 18V6h4v4h4v4h4v4" />
    </>
  ),
  beacon_light: (
    <>
      <path d="M12 4l2 6h6l-5 4 2 6-5-4-5 4 2-6-5-4h6l2-6z" />
    </>
  ),
  side_lights: (
    <>
      <path d="M4 10h4v4H4zM16 10h4v4h-4z" />
      {rays}
    </>
  ),
  turn_signal_left: (
    <>
      <path d="M14 8H8v4H5l4 4-4 4v-4h6V8z" />
    </>
  ),
  turn_signal_right: (
    <>
      <path d="M10 8h6v4h3l-4 4 4 4v-4H10V8z" />
    </>
  ),
};

export function isKnownFunctionIcon(slug: string): boolean {
  if (slug === "unspecified") {
    return true;
  }
  return (
    slug in GLYPHS || hasArtworkFunctionGlyph(slug) || hasFunctionIconAsset(slug)
  );
}

function isUnknownFunctionIcon(slug: string): boolean {
  if (
    slug === "unspecified" ||
    hasArtworkFunctionGlyph(slug) ||
    hasFunctionIconAsset(slug)
  ) {
    return false;
  }
  return !(slug in GLYPHS);
}

export function FunctionIconGlyph({
  slug,
  size = 24,
  variant = "default",
  active = false,
  style,
  className,
}: {
  slug: FunctionIconSlug;
  size?: number;
  variant?: "default" | "cockpit";
  active?: boolean;
  style?: CSSProperties;
  className?: string;
}) {
  const dimmed = variant === "cockpit" && !active;

  if (hasArtworkFunctionGlyph(slug)) {
    return (
      <ArtworkFunctionGlyph
        slug={slug}
        size={size}
        dimmed={dimmed}
        style={style}
        className={className}
      />
    );
  }

  const assetUrl = functionIconAssetUrl(slug);

  if (assetUrl) {
    return (
      <AssetFunctionIcon
        src={assetUrl}
        size={size}
        dimmed={dimmed}
        style={style}
        className={className}
      />
    );
  }

  const color =
    variant === "cockpit"
      ? active
        ? cockpit.text
        : "rgba(232, 240, 252, 0.72)"
      : undefined;

  if (isUnknownFunctionIcon(slug)) {
    return (
      <UnknownFunctionGlyph
        size={size}
        dimmed={dimmed}
        style={{ color, flexShrink: 0, ...style }}
        className={className}
      />
    );
  }

  return (
    <Svg
      size={size}
      style={{ color, flexShrink: 0, ...style }}
      className={className}
    >
      {GLYPHS[slug]}
    </Svg>
  );
}

/** Cockpit function button icon size (64px cell). */
export const COCKPIT_FUNCTION_ICON_PX = 30;
