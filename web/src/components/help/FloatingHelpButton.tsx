import { useCallback, useEffect, useRef, useState } from "react";
import { Fab } from "@mui/material";
import HelpOutlineIcon from "@mui/icons-material/HelpOutline";
import { useTranslation } from "react-i18next";

import { useHelpVisibility } from "../../hooks/useHelpVisibility";
import HelpDialog from "./HelpDialog";

const FAB_SIZE = 56;
const DRAG_THRESHOLD_MOUSE_PX = 6;
const DRAG_THRESHOLD_TOUCH_PX = 16;

function dragThresholdFor(pointerType: string): number {
  return pointerType === "touch" ? DRAG_THRESHOLD_TOUCH_PX : DRAG_THRESHOLD_MOUSE_PX;
}

function clampPosition(
  x: number,
  y: number,
  size = FAB_SIZE,
): { x: number; y: number } {
  const maxX = Math.max(0, window.innerWidth - size);
  const maxY = Math.max(0, window.innerHeight - size);
  return {
    x: Math.min(Math.max(0, x), maxX),
    y: Math.min(Math.max(0, y), maxY),
  };
}

export default function FloatingHelpButton() {
  const { t } = useTranslation(["help", "common"]);
  const {
    entry,
    pathname,
    visible,
    position,
    setPosition,
    disableRoute,
    disableGlobal,
    openRequestId,
    routeDisabled,
    globalDisabled,
  } = useHelpVisibility();
  const [dialogOpen, setDialogOpen] = useState(false);
  const lastOpenRequest = useRef(0);

  useEffect(() => {
    if (openRequestId === 0 || openRequestId === lastOpenRequest.current) {
      return;
    }
    lastOpenRequest.current = openRequestId;
    if (entry) {
      setDialogOpen(true);
    }
  }, [openRequestId, entry]);

  // pressing: finger/mouse down, not yet a drag. moved: threshold crossed → drag.
  const pressing = useRef(false);
  const moved = useRef(false);
  const start = useRef({
    pointerX: 0,
    pointerY: 0,
    originX: 0,
    originY: 0,
    threshold: DRAG_THRESHOLD_MOUSE_PX,
  });
  const posRef = useRef(position);
  posRef.current = position;

  useEffect(() => {
    const onResize = () => {
      setPosition(clampPosition(posRef.current.x, posRef.current.y));
    };
    window.addEventListener("resize", onResize);
    onResize();
    return () => window.removeEventListener("resize", onResize);
  }, [setPosition]);

  const onPointerDown = useCallback(
    (e: React.PointerEvent<HTMLButtonElement>) => {
      if (e.button !== 0) return;
      pressing.current = true;
      moved.current = false;
      start.current = {
        pointerX: e.clientX,
        pointerY: e.clientY,
        originX: posRef.current.x,
        originY: posRef.current.y,
        threshold: dragThresholdFor(e.pointerType),
      };
      e.currentTarget.setPointerCapture(e.pointerId);
    },
    [],
  );

  const onPointerMove = useCallback(
    (e: React.PointerEvent<HTMLButtonElement>) => {
      if (!pressing.current) return;
      const dx = e.clientX - start.current.pointerX;
      const dy = e.clientY - start.current.pointerY;
      if (!moved.current) {
        if (
          Math.abs(dx) <= start.current.threshold &&
          Math.abs(dy) <= start.current.threshold
        ) {
          // Still a tap candidate — keep FAB still.
          return;
        }
        moved.current = true;
      }
      setPosition(
        clampPosition(start.current.originX + dx, start.current.originY + dy),
      );
    },
    [setPosition],
  );

  const onPointerUp = useCallback(
    (e: React.PointerEvent<HTMLButtonElement>) => {
      if (!pressing.current) return;
      pressing.current = false;
      try {
        e.currentTarget.releasePointerCapture(e.pointerId);
      } catch {
        /* already released */
      }
      if (!moved.current) {
        setDialogOpen(true);
      }
    },
    [],
  );

  // Keep dialog mountable when opening from the account menu after a re-enable,
  // even for one frame before `visible` flips true.
  if ((!visible && !dialogOpen) || !entry) return null;

  return (
    <>
      {visible ? (
        <Fab
          color="primary"
          size="medium"
          aria-label={t("help:dialog.title")}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={onPointerUp}
          sx={{
            position: "fixed",
            left: position.x,
            top: position.y,
            zIndex: 1150,
            touchAction: "none",
            cursor: "grab",
            "&:active": { cursor: "grabbing" },
          }}
        >
          <HelpOutlineIcon />
        </Fab>
      ) : null}
      <HelpDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        entry={entry}
        pathname={pathname}
        routeDisabled={routeDisabled}
        globalDisabled={globalDisabled}
        onDisableRoute={() => {
          disableRoute(pathname);
          setDialogOpen(false);
        }}
        onDisableGlobal={() => {
          disableGlobal();
          setDialogOpen(false);
        }}
      />
    </>
  );
}
