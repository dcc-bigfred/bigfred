import { useEffect, useState } from "react";
import { Typography, type TypographyProps } from "@mui/material";

function formatRemaining(ms: number): string {
  if (ms <= 0) {
    return "00:00";
  }
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) {
    return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  }
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export interface LeaseCountdownProps extends Omit<TypographyProps, "children"> {
  expiresAt: string;
  expiredLabel?: string;
  warnBelowSec?: number;
}

export default function LeaseCountdown({
  expiresAt,
  expiredLabel,
  warnBelowSec = 60,
  ...typographyProps
}: LeaseCountdownProps) {
  const [remaining, setRemaining] = useState(() =>
    new Date(expiresAt).getTime() - Date.now(),
  );

  useEffect(() => {
    const tick = () => setRemaining(new Date(expiresAt).getTime() - Date.now());
    tick();
    const id = window.setInterval(tick, 1000);
    return () => window.clearInterval(id);
  }, [expiresAt]);

  const expired = remaining <= 0;
  const warn = !expired && remaining <= warnBelowSec * 1000;

  return (
    <Typography
      component="span"
      variant="body2"
      fontVariantNumeric="tabular-nums"
      color={expired ? "error" : warn ? "warning.main" : "text.primary"}
      {...typographyProps}
    >
      {expired ? (expiredLabel ?? "00:00") : formatRemaining(remaining)}
    </Typography>
  );
}
