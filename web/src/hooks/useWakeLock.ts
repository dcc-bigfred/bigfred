import { useEffect } from "react";
import NoSleep from "nosleep.js";

// useWakeLock keeps the device screen awake while the app tab is visible.
//
// We use NoSleep.js rather than the native Screen Wake Lock API on purpose:
// BigFred is served over plain HTTP on the local WiFi (never HTTPS), so it is
// not a secure context and `navigator.wakeLock` is unavailable on the phones.
// NoSleep.js falls back to a muted looping <video>, which keeps the screen on
// over HTTP, and transparently uses the native Wake Lock API when it *is*
// available (e.g. if the panel is ever served over HTTPS or via localhost).
//
// Browsers require a user gesture before media can play, so the first
// enable() is deferred to the first pointer/key interaction. The video is
// paused when the tab is hidden, so we re-enable on visibilitychange.
export function useWakeLock(enabled = true): void {
  useEffect(() => {
    if (!enabled || typeof navigator === "undefined") {
      return;
    }

    const noSleep = new NoSleep();
    let cancelled = false;
    let armed = false;

    const enable = () => {
      if (cancelled || document.visibilityState !== "visible") return;
      try {
        const result = noSleep.enable();
        if (result && typeof result.then === "function") {
          result.catch(() => {
            // Low battery, denied, or autoplay policy — ignore.
          });
        }
      } catch {
        // Not yet allowed (needs a user gesture) — retry on interaction.
      }
    };

    const onVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        if (armed) enable();
      } else {
        noSleep.disable();
      }
    };

    const onFirstInteraction = () => {
      armed = true;
      enable();
      document.removeEventListener("pointerdown", onFirstInteraction);
      document.removeEventListener("keydown", onFirstInteraction);
    };

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
      noSleep.disable();
    };
  }, [enabled]);
}
