import { describe, expect, it } from "vitest";

import { applyThrottleHardwareKeyStep } from "./volumeKeyStepSize";

describe("applyThrottleHardwareKeyStep", () => {
  it("increases and decreases by 5% of maxSpeed", () => {
    expect(applyThrottleHardwareKeyStep(1, 10, 127)).toBe(16);
    expect(applyThrottleHardwareKeyStep(-1, 16, 127)).toBe(10);
  });

  it("clamps to 0 and maxSpeed", () => {
    expect(applyThrottleHardwareKeyStep(-1, 0, 127)).toBe(0);
    expect(applyThrottleHardwareKeyStep(1, 125, 127)).toBe(127);
  });

  it("ignores zero direction", () => {
    expect(applyThrottleHardwareKeyStep(0, 42, 127)).toBe(42);
  });

  it("accumulates consecutive steps without waiting for React", () => {
    let speed = 0;
    speed = applyThrottleHardwareKeyStep(1, speed, 127);
    speed = applyThrottleHardwareKeyStep(1, speed, 127);
    speed = applyThrottleHardwareKeyStep(1, speed, 127);
    expect(speed).toBe(18);
  });
});
