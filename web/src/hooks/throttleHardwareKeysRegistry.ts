import { setThrottleHardwareKeysActive } from "../native/bigfredNativeApp";

export type ThrottleHardwareKeyHandler = (direction: number) => void;

const hardwareKeysStack: ThrottleHardwareKeyHandler[] = [];

function installTop(): void {
  const active = hardwareKeysStack.length > 0;
  if (typeof window !== "undefined") {
    window.__bigfredThrottleHardwareKeys = active
      ? (direction) =>
          hardwareKeysStack[hardwareKeysStack.length - 1]!(direction)
      : undefined;
  }
  // Tell the Android shell to consume volume keys only while a throttle
  // surface has a handler registered (Throttle page / takeover overlay).
  setThrottleHardwareKeysActive(active);
}

/** Push a handler; top of stack receives native volume key events. */
export function pushThrottleHardwareKeysHandler(
  handler: ThrottleHardwareKeyHandler,
): void {
  hardwareKeysStack.push(handler);
  installTop();
}

/** Remove a handler; restores the previous one if any. */
export function popThrottleHardwareKeysHandler(
  handler: ThrottleHardwareKeyHandler,
): void {
  const index = hardwareKeysStack.lastIndexOf(handler);
  if (index >= 0) {
    hardwareKeysStack.splice(index, 1);
  }
  installTop();
}

/** @internal Test helper — clears the stack. */
export function resetThrottleHardwareKeysRegistryForTests(): void {
  hardwareKeysStack.length = 0;
  installTop();
}
