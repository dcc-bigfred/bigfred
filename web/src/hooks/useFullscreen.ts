import { useCallback, useEffect, useState } from "react";

// useFullscreen toggles the browser fullscreen API on documentElement.
export function useFullscreen() {
  const [active, setActive] = useState(
    () =>
      typeof document !== "undefined" && document.fullscreenElement != null,
  );

  useEffect(() => {
    const onChange = () => {
      setActive(document.fullscreenElement != null);
    };
    document.addEventListener("fullscreenchange", onChange);
    return () => document.removeEventListener("fullscreenchange", onChange);
  }, []);

  const toggle = useCallback(async () => {
    if (!document.fullscreenEnabled) {
      return;
    }
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else {
        await document.documentElement.requestFullscreen();
      }
    } catch {
      // User denied or platform blocked the request.
    }
  }, []);

  const supported =
    typeof document !== "undefined" && document.fullscreenEnabled;

  return { active, toggle, supported };
}
