import type { CSSProperties } from "react";

/** Renders a PNG from `web/src/icons/png/<slug>.png` (rasterized function artwork). */
export default function AssetFunctionIcon({
  src,
  size = 24,
  dimmed = false,
  style,
  className,
}: {
  src: string;
  size?: number;
  dimmed?: boolean;
  style?: CSSProperties;
  className?: string;
}) {
  return (
    <img
      src={src}
      width={size}
      height={size}
      alt=""
      aria-hidden
      draggable={false}
      className={className}
      style={{
        display: "block",
        flexShrink: 0,
        objectFit: "contain",
        opacity: dimmed ? 0.72 : 1,
        ...style,
      }}
    />
  );
}
