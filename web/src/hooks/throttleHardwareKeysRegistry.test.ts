import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  popThrottleHardwareKeysHandler,
  pushThrottleHardwareKeysHandler,
  resetThrottleHardwareKeysRegistryForTests,
} from "./throttleHardwareKeysRegistry";

beforeEach(() => {
  if (typeof globalThis.window === "undefined") {
    // Vitest node environment has no DOM window.
    // @ts-expect-error test shim
    globalThis.window = globalThis;
  }
  window.BigFredNativeApp = {
    openModelPicker: () => {},
    setThrottleHardwareKeysActive: vi.fn(),
  };
});

afterEach(() => {
  resetThrottleHardwareKeysRegistryForTests();
});

describe("throttleHardwareKeysRegistry", () => {
  it("routes events to the top handler on the stack", () => {
    const calls: number[] = [];
    const bottom = (d: number) => calls.push(d * 10);
    const top = (d: number) => calls.push(d);

    pushThrottleHardwareKeysHandler(bottom);
    pushThrottleHardwareKeysHandler(top);

    window.__bigfredThrottleHardwareKeys?.(1);
    expect(calls).toEqual([1]);
  });

  it("restores the previous handler after pop", () => {
    const calls: number[] = [];
    const bottom = (d: number) => calls.push(d * 10);
    const top = (d: number) => calls.push(d);

    pushThrottleHardwareKeysHandler(bottom);
    pushThrottleHardwareKeysHandler(top);
    popThrottleHardwareKeysHandler(top);

    window.__bigfredThrottleHardwareKeys?.(2);
    expect(calls).toEqual([20]);
  });

  it("clears the global when the stack is empty", () => {
    const handler = vi.fn();
    pushThrottleHardwareKeysHandler(handler);
    popThrottleHardwareKeysHandler(handler);

    expect(window.__bigfredThrottleHardwareKeys).toBeUndefined();
  });

  it("notifies the native shell when the stack becomes active or empty", () => {
    const notify = window.BigFredNativeApp!.setThrottleHardwareKeysActive as ReturnType<
      typeof vi.fn
    >;
    const handler = vi.fn();

    pushThrottleHardwareKeysHandler(handler);
    expect(notify).toHaveBeenLastCalledWith(true);

    popThrottleHardwareKeysHandler(handler);
    expect(notify).toHaveBeenLastCalledWith(false);
  });
});
