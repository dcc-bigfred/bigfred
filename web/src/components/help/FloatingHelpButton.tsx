import { useCallback, useEffect, useRef, useState } from "react";
import { Fab } from "@mui/material";
import HelpOutlineIcon from "@mui/icons-material/HelpOutline";
import { useTranslation } from "react-i18next";

import { useHelpVisibility } from "../../hooks/useHelpVisibility";
import HelpDialog from "./HelpDialog";

const FAB_SIZE = 56;
const DRAG_THRESHOLD_PX = 6;

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
  } = useHelpVisibility();
  const [dialogOpen, setDialogOpen] = useState(false);

  const dragging = useRef(false);
  const moved = useRef(false);
  const start = useRef({ pointerX: 0, pointerY: 0, originX: 0, originY: 0 });
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
      dragging.current = true;
      moved.current = false;
      start.current = {
        pointerX: e.clientX,
        pointerY: e.clientY,
        originX: posRef.current.x,
        originY: posRef.current.y,
      };
      e.currentTarget.setPointerCapture(e.pointerId);
    },
    [],
  );

  const onPointerMove = useCallback(
    (e: React.PointerEvent<HTMLButtonElement>) => {
      if (!dragging.current) return;
      const dx = e.clientX - start.current.pointerX;
      const dy = e.clientY - start.current.pointerY;
      if (
        Math.abs(dx) > DRAG_THRESHOLD_PX ||
        Math.abs(dy) > DRAG_THRESHOLD_PX
      ) {
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
      if (!dragging.current) return;
      dragging.current = false;
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

  if (!visible || !entry) return null;

  return (
    <>
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
      <HelpDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        entry={entry}
        pathname={pathname}
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
