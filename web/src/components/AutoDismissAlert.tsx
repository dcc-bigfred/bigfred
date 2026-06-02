import { useEffect, useState } from "react";
import { Alert, type AlertProps } from "@mui/material";

const DEFAULT_HIDE_MS = 2000;

export interface AutoDismissAlertProps extends AlertProps {
  /** Remount timer when this value changes (e.g. error code). */
  resetKey?: string | number | boolean | null;
  autoHideMs?: number;
}

// AutoDismissAlert hides itself after a short delay (toast-like).
export default function AutoDismissAlert({
  resetKey,
  autoHideMs = DEFAULT_HIDE_MS,
  onClose,
  ...alertProps
}: AutoDismissAlertProps) {
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    setVisible(true);
  }, [resetKey]);

  useEffect(() => {
    if (!visible) {
      return;
    }
    const timer = window.setTimeout(() => setVisible(false), autoHideMs);
    return () => window.clearTimeout(timer);
  }, [visible, autoHideMs, onClose, resetKey]);

  if (!visible) {
    return null;
  }

  return <Alert {...alertProps} onClose={onClose} />;
}
