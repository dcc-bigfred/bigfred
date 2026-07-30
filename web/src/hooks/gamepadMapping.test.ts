import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  defaultGamepadMapping,
  loadGamepadMapping,
  loadLastGamepadMapping,
  saveGamepadMapping,
  type GamepadMapping,
} from "./gamepadMapping";

const STORAGE_KEY_PREFIX = "bigfred.throttle.gamepad";
const LAST_MAPPING_KEY = `${STORAGE_KEY_PREFIX}.last`;

function perIdKey(gamepadId: string): string {
  return `${STORAGE_KEY_PREFIX}.${encodeURIComponent(gamepadId)}`;
}

function createMemoryStorage(): Storage {
  const store = new Map<string, string>();
  return {
    get length() {
      return store.size;
    },
    clear() {
      store.clear();
    },
    getItem(key: string) {
      return store.has(key) ? store.get(key)! : null;
    },
    key(index: number) {
      return [...store.keys()][index] ?? null;
    },
    removeItem(key: string) {
      store.delete(key);
    },
    setItem(key: string, value: string) {
      store.set(key, String(value));
    },
  };
}

function sampleMapping(
  gamepadId: string,
  overrides: Partial<GamepadMapping> = {},
): GamepadMapping {
  return {
    ...defaultGamepadMapping(gamepadId),
    idleAxisMin: -0.05,
    idleAxisMax: 0.08,
    reverseButton: 1,
    stopButton: 0,
    fnButtons: { 0: 2 },
    enabled: true,
    ...overrides,
  };
}

beforeEach(() => {
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: createMemoryStorage(),
  });
});

afterEach(() => {
  localStorage.clear();
});

describe("gamepadMapping persistence", () => {
  it("defaults new mappings to enabled=true", () => {
    expect(defaultGamepadMapping("pad-a").enabled).toBe(true);
  });

  it("saves both per-id and last-mapping keys", () => {
    const mapping = sampleMapping("Xbox Controller");
    saveGamepadMapping(mapping);

    expect(localStorage.getItem(perIdKey("Xbox Controller"))).toBeTruthy();
    expect(localStorage.getItem(LAST_MAPPING_KEY)).toBeTruthy();
    expect(loadLastGamepadMapping()).toMatchObject({
      gamepadId: "Xbox Controller",
      enabled: true,
      idleAxisMin: -0.05,
      reverseButton: 1,
    });
  });

  it("loads an existing per-id mapping (migration / exact match)", () => {
    const legacy = sampleMapping("Legacy Pad", {
      enabled: true,
      accelerateButton: 5,
    });
    localStorage.setItem(perIdKey("Legacy Pad"), JSON.stringify(legacy));

    const loaded = loadGamepadMapping("Legacy Pad");
    expect(loaded.gamepadId).toBe("Legacy Pad");
    expect(loaded.accelerateButton).toBe(5);
    expect(loaded.enabled).toBe(true);
  });

  it("falls back to last mapping when Gamepad.id changes", () => {
    saveGamepadMapping(sampleMapping("id-v1", { stopButton: 9, enabled: true }));

    const loaded = loadGamepadMapping("id-v2-changed-by-webview");
    expect(loaded.gamepadId).toBe("id-v2-changed-by-webview");
    expect(loaded.stopButton).toBe(9);
    expect(loaded.enabled).toBe(true);
    expect(loaded.idleAxisMin).toBe(-0.05);
  });

  it("prefers exact per-id mapping over last mapping", () => {
    saveGamepadMapping(sampleMapping("pad-a", { stopButton: 1 }));
    // Saving pad-b overwrites LAST, but pad-a per-id key must still win.
    saveGamepadMapping(sampleMapping("pad-b", { stopButton: 7 }));

    expect(loadGamepadMapping("pad-a").stopButton).toBe(1);
    expect(loadGamepadMapping("pad-b").stopButton).toBe(7);
  });

  it("respects historically stored enabled=false", () => {
    const disabled = sampleMapping("pad-off", { enabled: false });
    localStorage.setItem(perIdKey("pad-off"), JSON.stringify(disabled));
    localStorage.setItem(LAST_MAPPING_KEY, JSON.stringify(disabled));

    expect(loadGamepadMapping("pad-off").enabled).toBe(false);
    expect(loadLastGamepadMapping()?.enabled).toBe(false);

    // Fallback after id change must also keep enabled=false.
    expect(loadGamepadMapping("pad-off-renamed").enabled).toBe(false);
  });

  it("keeps idle calibration and buttons when saving enabled=false", () => {
    const mapping = sampleMapping("pad-cal", {
      enabled: false,
      idleAxisMin: -0.1,
      idleAxisMax: 0.12,
      reverseButton: 3,
      fnButtons: { 1: 4 },
    });
    saveGamepadMapping(mapping);

    const loaded = loadGamepadMapping("pad-cal");
    expect(loaded.enabled).toBe(false);
    expect(loaded.idleAxisMin).toBe(-0.1);
    expect(loaded.idleAxisMax).toBe(0.12);
    expect(loaded.reverseButton).toBe(3);
    expect(loaded.fnButtons).toEqual({ 1: 4 });
  });

  it("loadLastGamepadMapping returns null when nothing was saved", () => {
    expect(loadLastGamepadMapping()).toBeNull();
  });

  it("returns defaults when neither per-id nor last mapping exist", () => {
    const loaded = loadGamepadMapping("brand-new");
    expect(loaded).toEqual(defaultGamepadMapping("brand-new"));
    expect(loaded.enabled).toBe(true);
  });
});
