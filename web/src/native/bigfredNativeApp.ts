/** Detects the BigFred native mobile shell and talks to the JS bridge. */

export interface ModelPickPayload {
  vehicleNumber?: string | null;
  carrier?: string | null;
  assignment?: string | null;
  revisionDate?: string | null;
  epochs?: string[] | null;
}

interface BigFredNativeAppBridge {
  openModelPicker: () => void;
  getPreferredLocale?: () => string;
}

declare global {
  interface Window {
    BigFredNativeApp?: BigFredNativeAppBridge;
    __bigfredOnModelPicked?: (payload: ModelPickPayload | null) => void;
    __bigfredSetLocale?: (lang: string) => void;
    /** Native volume hardware keys: +1 faster, -1 slower. Registered by throttle pages. */
    __bigfredThrottleHardwareKeys?: (direction: number) => void;
  }
}

/** True when running inside the BigFred native mobile WebView (UA suffix). */
export function isBigFredNativeMobileApp(): boolean {
  if (typeof navigator === "undefined") return false;
  return /BigFredNativeApp\//.test(navigator.userAgent);
}

/** Preferred locale from the native shell, or null if unavailable. */
export function getPreferredLocale(): string | null {
  const bridge = window.BigFredNativeApp;
  if (typeof bridge?.getPreferredLocale !== "function") {
    return null;
  }
  try {
    const lang = bridge.getPreferredLocale()?.trim();
    return lang || null;
  } catch {
    return null;
  }
}

/**
 * Opens the native model catalogue picker. Resolves with the selected
 * model payload, or null if the user cancelled / bridge is unavailable.
 */
export function openModelPicker(): Promise<ModelPickPayload | null> {
  const bridge = window.BigFredNativeApp;
  if (typeof bridge?.openModelPicker !== "function") {
    return Promise.resolve(null);
  }
  return new Promise((resolve) => {
    window.__bigfredOnModelPicked = (payload) => {
      window.__bigfredOnModelPicked = undefined;
      resolve(payload ?? null);
    };
    try {
      bridge.openModelPicker();
    } catch {
      window.__bigfredOnModelPicked = undefined;
      resolve(null);
    }
  });
}
