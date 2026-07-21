import {
  memo,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  type PointerEvent as ReactPointerEvent,
} from "react";
import { Box } from "@mui/material";

import { cockpit } from "./throttleCockpitTheme";

/** Thumb height in px — must match the thumb Box height below. */
export const THUMB_HEIGHT_PX = 50;

interface VerticalThrottleProps {
  value: number;
  max: number;
  disabled?: boolean;
  onChange: (value: number) => void;
}

function ratioOf(value: number, max: number): number {
  return max > 0 ? Math.min(1, Math.max(0, value / max)) : 0;
}

/**
 * Map ratio (0 = bottom / stop, 1 = top / full) to a compositor-friendly
 * transform. translateY must be in px based on track height — CSS % in
 * translate refers to the thumb's own size, not the track.
 *
 * With top:0, the thumb's natural centre is at THUMB_HEIGHT_PX/2.
 * Target centre is H * (1 - ratio). So:
 *   translateY = H * (1 - ratio) - THUMB_HEIGHT_PX / 2
 *
 * Exported for unit tests (0 -> bottom, 1 -> top, midpoints proportional).
 */
export function thumbTransformPx(ratio: number, trackHeight: number): string {
  const y = trackHeight * (1 - ratio) - THUMB_HEIGHT_PX / 2;
  return `translate(-50%, ${y}px)`;
}

// VerticalThrottle is a custom vertical speed lever (bottom = 0).
// During drag the thumb is moved imperatively via transform (GPU compositor)
// and onChange is committed at most once per animation frame with value dedupe,
// so the parent tree is not re-rendered on every pointermove.
function VerticalThrottle({
  value,
  max,
  disabled,
  onChange,
}: VerticalThrottleProps) {
  const trackRef = useRef<HTMLDivElement>(null);
  const thumbRef = useRef<HTMLDivElement>(null);
  const valueRef = useRef(value);
  const maxRef = useRef(max);
  const onChangeRef = useRef(onChange);
  const disabledRef = useRef(disabled);
  const draggingRef = useRef(false);
  const lastCommittedRef = useRef(value);
  const pendingValueRef = useRef<number | null>(null);
  const rafRef = useRef<number | null>(null);
  const cleanupDragRef = useRef<(() => void) | null>(null);
  // Cached track geometry — updated by ResizeObserver and refreshed at the
  // start of each drag. Reading it avoids getBoundingClientRect() on every
  // pointermove (a layout-triggering call in the drag hot path).
  const trackGeomRef = useRef<{ top: number; height: number } | null>(null);

  valueRef.current = value;
  maxRef.current = max;
  onChangeRef.current = onChange;
  disabledRef.current = disabled;

  const refreshTrackGeom = useCallback(() => {
    const track = trackRef.current;
    if (!track) return;
    const rect = track.getBoundingClientRect();
    if (rect.height <= 0) return;
    trackGeomRef.current = { top: rect.top, height: rect.height };
  }, []);

  const applyThumb = useCallback((ratio: number) => {
    const thumb = thumbRef.current;
    const geom = trackGeomRef.current;
    if (!thumb || !geom) return;
    // Imperative only — keep transform out of React/MUI sx so re-renders
    // (prop sync while dragging) cannot overwrite the live drag position.
    thumb.style.transform = thumbTransformPx(ratio, geom.height);
  }, []);

  // Initial + non-drag sync from props (server echo / external stop).
  useLayoutEffect(() => {
    if (draggingRef.current) return;
    refreshTrackGeom();
    applyThumb(ratioOf(value, max));
    lastCommittedRef.current = value;
  }, [value, max, applyThumb, refreshTrackGeom]);

  // Keep cached track geometry fresh and re-apply the thumb position when
  // the track is resized (rotation, chrome show/hide). Pixel positions do
  // not auto-follow track size the way % bottom did.
  useEffect(() => {
    const track = trackRef.current;
    if (!track || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => {
      refreshTrackGeom();
      if (draggingRef.current) return;
      applyThumb(ratioOf(valueRef.current, maxRef.current));
    });
    observer.observe(track);
    return () => observer.disconnect();
  }, [applyThumb, refreshTrackGeom]);

  const flushPending = useCallback(() => {
    rafRef.current = null;
    const pending = pendingValueRef.current;
    if (pending == null) return;
    pendingValueRef.current = null;
    if (pending === lastCommittedRef.current) return;
    lastCommittedRef.current = pending;
    onChangeRef.current(pending);
  }, []);

  const scheduleCommit = useCallback(
    (next: number) => {
      pendingValueRef.current = next;
      if (rafRef.current != null) return;
      rafRef.current = requestAnimationFrame(flushPending);
    },
    [flushPending],
  );

  const valueFromClientY = useCallback((clientY: number) => {
    const geom = trackGeomRef.current;
    const maxV = maxRef.current;
    if (!geom || maxV <= 0) return 0;
    const rel = 1 - (clientY - geom.top) / geom.height;
    const clamped = Math.min(1, Math.max(0, rel));
    return Math.round(clamped * maxV);
  }, []);

  const endDrag = useCallback(() => {
    draggingRef.current = false;
    if (rafRef.current != null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    const pending = pendingValueRef.current;
    pendingValueRef.current = null;
    if (pending != null && pending !== lastCommittedRef.current) {
      lastCommittedRef.current = pending;
      onChangeRef.current(pending);
    }
    cleanupDragRef.current?.();
    cleanupDragRef.current = null;
  }, []);

  const startDrag = useCallback(
    (clientY: number) => {
      if (disabledRef.current) return;

      draggingRef.current = true;
      // Refresh geometry once at drag start so pointermove never reads layout.
      refreshTrackGeom();
      const next = valueFromClientY(clientY);
      applyThumb(ratioOf(next, maxRef.current));
      scheduleCommit(next);

      const onMove = (ev: PointerEvent) => {
        const v = valueFromClientY(ev.clientY);
        applyThumb(ratioOf(v, maxRef.current));
        scheduleCommit(v);
      };
      const onUp = () => {
        endDrag();
      };
      const onCancel = () => {
        endDrag();
      };

      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onUp);
      window.addEventListener("pointercancel", onCancel);

      cleanupDragRef.current = () => {
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onUp);
        window.removeEventListener("pointercancel", onCancel);
      };
    },
    [applyThumb, endDrag, refreshTrackGeom, scheduleCommit, valueFromClientY],
  );

  useEffect(
    () => () => {
      if (rafRef.current != null) {
        cancelAnimationFrame(rafRef.current);
      }
      cleanupDragRef.current?.();
    },
    [],
  );

  const handlePointerDown = (ev: ReactPointerEvent<HTMLDivElement>) => {
    if (disabled) return;
    // Avoid focus ring / selection flash on custom drag surface.
    ev.preventDefault();
    try {
      ev.currentTarget.setPointerCapture(ev.pointerId);
    } catch {
      // Older WebViews may throw; window listeners still drive the drag.
    }
    startDrag(ev.clientY);
  };

  const handleLostPointerCapture = () => {
    if (draggingRef.current) {
      endDrag();
    }
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
      onLostPointerCapture={handleLostPointerCapture}
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
        ref={thumbRef}
        sx={{
          position: "absolute",
          left: "50%",
          top: 0,
          width: "85%",
          height: THUMB_HEIGHT_PX,
          borderRadius: THUMB_HEIGHT_PX / 2,
          bgcolor: cockpit.thumb,
          background: `linear-gradient(180deg, #3a3a3a 0%, ${cockpit.thumb} 55%, #000 100%)`,
          boxShadow: `0 0 0 1px rgba(255,255,255,0.15), inset 0 2px 0 ${cockpit.thumbHighlight}, 0 4px 8px rgba(0,0,0,0.5)`,
          pointerEvents: "none",
          willChange: "transform",
          backfaceVisibility: "hidden",
        }}
      />
    </Box>
  );
}

export default memo(VerticalThrottle);
