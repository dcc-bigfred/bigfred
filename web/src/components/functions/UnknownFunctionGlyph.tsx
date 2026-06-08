import type { CSSProperties } from "react";

// Letter “F” from web/src/icons/unknown-func.svg (tile background omitted).
const VIEW_BOX = "14 10 26 30";

const LETTER_PATH =
  "m 23.332276,12.200613 c 4.53887,-0.3048 10.97471,-0.171053 15.48043,0.281186 1.66608,0.167204 1.12197,4.424918 0.0829,5.827951 -3.44567,2.460823 -10.07483,-2.574965 -11.91721,2.869208 -0.12037,0.355679 -0.0704,2.85234 -0.0637,3.371691 2.7751,-0.124421 8.12808,-0.677148 8.82618,3.119913 -0.27265,2.31783 -2.41538,2.779356 -4.32963,2.891989 l -4.45643,-0.221734 -0.0768,14.686359 -7.057623,-0.0028 c -0.03621,-7.516791 -0.114399,-15.049637 -0.04741,-22.564346 0.03675,-4.119443 -0.47621,-8.117204 3.55929,-10.259417 z";

export default function UnknownFunctionGlyph({
  size = 24,
  dimmed = false,
  style,
  className,
}: {
  size?: number;
  dimmed?: boolean;
  style?: CSSProperties;
  className?: string;
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox={VIEW_BOX}
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
      className={className}
      style={{
        flexShrink: 0,
        display: "block",
        color: "currentColor",
        opacity: dimmed ? 0.72 : 1,
        ...style,
      }}
    >
      <path fill="currentColor" d={LETTER_PATH} />
    </svg>
  );
}
