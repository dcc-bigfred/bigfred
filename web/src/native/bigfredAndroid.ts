/** Detects the BigFred Android WebView shell and opens the native model picker. */

export interface ModelPickPayload {
  vehicleNumber?: string | null;
  carrier?: string | null;
  assignment?: string | null;
  revisionDate?: string | null;
  epochs?: string[] | null;
}

interface BigFredAndroidBridge {
  openModelPicker: () => void;
}

declare global {
  interface Window {
    BigFredAndroid?: BigFredAndroidBridge;
    __bigfredOnModelPicked?: (payload: ModelPickPayload | null) => void;
  }
}

/** True when running inside the BigFred Android WebView (UA suffix). */
export function isBigFredAndroid(): boolean {
  if (typeof navigator === "undefined") return false;
  return /BigFredAndroid\//.test(navigator.userAgent);
}

/**
 * Opens the native model catalogue picker. Resolves with the selected
 * model payload, or null if the user cancelled / bridge is unavailable.
 */
export function openModelPicker(): Promise<ModelPickPayload | null> {
  const bridge = window.BigFredAndroid;
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
