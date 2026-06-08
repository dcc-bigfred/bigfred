import type { CSSProperties } from "react";

import {
  getArtworkFunctionGlyph,
  hasArtworkFunctionGlyph,
} from "./artworkFunctionGlyphs";

export { hasArtworkFunctionGlyph };

/** Full-colour Inkscape icon rendered as inline SVG paths (from `web/src/icons/<slug>.svg`). */
export default function ArtworkFunctionGlyph({
  slug,
  size = 24,
  dimmed = false,
  style,
  className,
}: {
  slug: string;
  size?: number;
  dimmed?: boolean;
  style?: CSSProperties;
  className?: string;
}) {
  const artwork = getArtworkFunctionGlyph(slug);
  if (!artwork) return null;

  return (
    <svg
      viewBox={artwork.viewBox}
      width={size}
      height={size}
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
      className={className}
      style={{
        display: "block",
        flexShrink: 0,
        opacity: dimmed ? 0.72 : 1,
        ...style,
      }}
    >
      {artwork.paths.map((path, index) => (
        <path
          key={index}
          fill={path.fill}
          d={path.d}
          transform={path.transform}
        />
      ))}
    </svg>
  );
}
