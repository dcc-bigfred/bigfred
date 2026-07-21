import { describe, expect, it } from "vitest";

import { thumbTransformPx } from "./VerticalThrottle";

const THUMB_HEIGHT_PX = 50;

function translateYOf(transform: string): number {
  // transform is `translate(-50%, <y>px)`
  const match = transform.match(/translate\(-50%,\s*(-?[\d.]+)px\)/);
  expect(match, `unexpected transform: ${transform}`).not.toBeNull();
  return Number(match![1]);
}

describe("thumbTransformPx", () => {
  it("places the thumb at the bottom of the track when ratio is 0", () => {
    const trackHeight = 400;
    const y = translateYOf(thumbTransformPx(0, trackHeight));
    // ratio 0 -> thumb centre at trackHeight (bottom edge).
    // translateY = H - THUMB_HEIGHT_PX/2
    expect(y).toBe(trackHeight - THUMB_HEIGHT_PX / 2);
    expect(y).toBe(375);
  });

  it("places the thumb at the top of the track when ratio is 1", () => {
    const trackHeight = 400;
    const y = translateYOf(thumbTransformPx(1, trackHeight));
    // ratio 1 -> thumb centre at 0 (top edge).
    // translateY = 0 - THUMB_HEIGHT_PX/2
    expect(y).toBe(-THUMB_HEIGHT_PX / 2);
    expect(y).toBe(-25);
  });

  it("places the thumb proportionally for a midpoint ratio", () => {
    const trackHeight = 400;
    const y = translateYOf(thumbTransformPx(0.5, trackHeight));
    // ratio 0.5 -> thumb centre at H/2 (middle).
    expect(y).toBe(trackHeight / 2 - THUMB_HEIGHT_PX / 2);
    expect(y).toBe(175);
  });

  it("scales with track height for the same ratio", () => {
    const yShort = translateYOf(thumbTransformPx(0.25, 200));
    const yTall = translateYOf(thumbTransformPx(0.25, 600));
    // ratio 0.25 -> centre at 0.75 * H
    expect(yShort).toBe(200 * 0.75 - THUMB_HEIGHT_PX / 2);
    expect(yTall).toBe(600 * 0.75 - THUMB_HEIGHT_PX / 2);
    expect(yTall).toBeGreaterThan(yShort);
  });

  it("clamps ratio outside [0,1] by the caller (function itself is linear)", () => {
    // The function is linear; clamping happens in ratioOf. Verify the math
    // for a ratio > 1 still follows the formula (caller guards input).
    const y = translateYOf(thumbTransformPx(1.5, 400));
    expect(y).toBe(400 * (1 - 1.5) - THUMB_HEIGHT_PX / 2);
    expect(y).toBe(-225);
  });
});
