import { describe, expect, it } from "vitest";
import { watermarkLines } from "./screenshot-watermark";

describe("watermarkLines", () => {
  it("uses the trusted display name and server timestamp as two watermark lines", () => {
    expect(watermarkLines({ watermarkEnabled: true, displayName: "凯撒（沙磊）", capturedAt: "2026-09-02 09:18:36" })).toEqual([
      "凯撒（沙磊）",
      "2026-09-02 09:18:36",
    ]);
  });

  it("rejects incomplete metadata when watermarking is enabled", () => {
    expect(() => watermarkLines({ watermarkEnabled: true, displayName: "", capturedAt: "" })).toThrow("截图水印信息不完整");
  });

  it("does not draw watermark lines when the global setting is disabled", () => {
    expect(watermarkLines({ watermarkEnabled: false })).toEqual([]);
  });
});
