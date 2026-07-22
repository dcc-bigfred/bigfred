export type ThrottleHardwareKeyHandler = (direction: number) => void;

const hardwareKeysStack: ThrottleHardwareKeyHandler[] = [];

function installTop(): void {
  window.__bigfredThrottleHardwareKeys =
    hardwareKeysStack.length > 0
      ? (direction) => hardwareKeysStack[hardwareKeysStack.length - 1]!(direction)
      : undefined;
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
