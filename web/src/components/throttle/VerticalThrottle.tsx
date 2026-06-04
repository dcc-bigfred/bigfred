import { useCallback, useRef, type PointerEvent as ReactPointerEvent } from "react";
import { Box } from "@mui/material";

import { cockpit } from "./throttleCockpitTheme";

interface VerticalThrottleProps {
  value: number;
  max: number;
  disabled?: boolean;
  onChange: (value: number) => void;
}

// VerticalThrottle is a custom vertical speed lever (bottom = 0).
export default function VerticalThrottle({
  value,
  max,
  disabled,
  onChange,
}: VerticalThrottleProps) {
  const trackRef = useRef<HTMLDivElement>(null);

  const ratio = max > 0 ? value / max : 0;
  const thumbBottom = `${ratio * 100}%`;

  const valueFromClientY = useCallback(
    (clientY: number) => {
      const track = trackRef.current;
      if (!track || max <= 0) {
        return 0;
      }
      const rect = track.getBoundingClientRect();
      const rel = 1 - (clientY - rect.top) / rect.height;
      const clamped = Math.min(1, Math.max(0, rel));
      return Math.round(clamped * max);
    },
    [max],
  );

  const startDrag = useCallback(
    (clientY: number) => {
      if (disabled) {
        return;
      }
      onChange(valueFromClientY(clientY));

      const onMove = (ev: PointerEvent) => {
        onChange(valueFromClientY(ev.clientY));
      };
      const onUp = () => {
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onUp);
      };
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onUp);
    },
    [disabled, onChange, valueFromClientY],
  );

  const handlePointerDown = (ev: ReactPointerEvent<HTMLDivElement>) => {
    if (disabled) {
      return;
    }
    // Avoid focus ring / selection flash on custom drag surface.
    ev.preventDefault();
    ev.currentTarget.setPointerCapture(ev.pointerId);
    startDrag(ev.clientY);
  };

  return (
    <Box
      ref={trackRef}
      role="slider"
      aria-valuemin={0}
      aria-valuemax={max}
      aria-valuenow={value}
      tabIndex={-1}
      onPointerDown={handlePointerDown}
      sx={{
        position: "relative",
        flex: 1,
        width: "100%",
        minHeight: 0,
        borderRadius: 2,
        bgcolor: cockpit.track,
        boxShadow: `inset 3px 0 0 ${cockpit.trackHighlight}, inset 0 2px 8px rgba(0,0,0,0.6)`,
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.45 : 1,
        touchAction: "none",
        userSelect: "none",
        WebkitUserSelect: "none",
        WebkitTapHighlightColor: "transparent",
        outline: "none",
        "&:focus": { outline: "none" },
        "&:focus-visible": { outline: "none" },
        "&::selection": { background: "transparent" },
      }}
    >
      <Box
        sx={{
          position: "absolute",
          left: "50%",
          bottom: thumbBottom,
          transform: "translate(-50%, 50%)",
          width: "85%",
          height: 50,
          borderRadius: 25,
          bgcolor: cockpit.thumb,
          background: `linear-gradient(180deg, #3a3a3a 0%, ${cockpit.thumb} 55%, #000 100%)`,
          boxShadow: `0 0 0 1px rgba(255,255,255,0.15), inset 0 2px 0 ${cockpit.thumbHighlight}, 0 4px 8px rgba(0,0,0,0.5)`,
          pointerEvents: "none",
          transition: "bottom 0.05s linear",
        }}
      />
    </Box>
  );
}
