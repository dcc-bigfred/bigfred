import { useEffect } from "react";

function wakeLockSupported(): boolean {
  return typeof navigator !== "undefined" && "wakeLock" in navigator;
}

// useWakeLock keeps the device screen awake while the app tab is visible.
// Uses the Screen Wake Lock API; silently no-ops when unsupported or denied.
//
// Browsers release the lock when the tab is hidden — we re-acquire on
// visibilitychange. Some mobile browsers (notably iOS Safari) only grant
// the first lock after a user gesture, so we also retry once on pointerdown.
export function useWakeLock(enabled = true): void {
  useEffect(() => {
    if (!enabled || !wakeLockSupported()) {
      return;
    }

    let lock: WakeLockSentinel | null = null;
    let cancelled = false;

    const release = async () => {
      if (!lock) return;
      try {
        await lock.release();
      } catch {
        // Already released.
      }
      lock = null;
    };

    const acquire = async () => {
      if (cancelled || document.visibilityState !== "visible") return;
      if (lock && !lock.released) return;
      try {
        lock = await navigator.wakeLock.request("screen");
        lock.addEventListener("release", () => {
          lock = null;
        });
      } catch {
        // Low battery, permission denied, or platform policy.
      }
    };

    const onVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        void acquire();
      } else {
        void release();
      }
    };

    const onFirstInteraction = () => {
      void acquire();
      document.removeEventListener("pointerdown", onFirstInteraction);
      document.removeEventListener("keydown", onFirstInteraction);
    };

    void acquire();
    document.addEventListener("visibilitychange", onVisibilityChange);
    document.addEventListener("pointerdown", onFirstInteraction, {
      passive: true,
    });
    document.addEventListener("keydown", onFirstInteraction);

    return () => {
      cancelled = true;
      document.removeEventListener("visibilitychange", onVisibilityChange);
      document.removeEventListener("pointerdown", onFirstInteraction);
      document.removeEventListener("keydown", onFirstInteraction);
      void release();
    };
  }, [enabled]);
}
