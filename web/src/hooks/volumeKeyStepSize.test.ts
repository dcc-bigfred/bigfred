import { describe, expect, it } from "vitest";

import { volumeKeyStepSize } from "./volumeKeyStepSize";

describe("volumeKeyStepSize", () => {
  it("is 5% of maxSpeed rounded, minimum 1", () => {
    expect(volumeKeyStepSize(127)).toBe(6);
    expect(volumeKeyStepSize(28)).toBe(1);
    expect(volumeKeyStepSize(15)).toBe(1);
    expect(volumeKeyStepSize(0)).toBe(1);
  });
});
