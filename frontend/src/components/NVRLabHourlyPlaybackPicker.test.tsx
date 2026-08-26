import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NVRLabHourlyPlaybackPicker } from "../pages/NVRLabCamera";

describe("NVRLabHourlyPlaybackPicker", () => {
  it("renders the shared replay time picker, hourly shortcuts and selected range", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabHourlyPlaybackPicker, {
        startAt: "2026-08-25T11:00",
        range: {
          startAt: "2026-08-25T11:00",
          endAt: "2026-08-25T12:00",
          startTime: 1,
          endTime: 3601,
        },
        onStartAtChange: () => undefined,
        onConfirm: () => undefined,
      }),
    );

    expect(markup).toContain("回放时间");
    expect(markup).toContain("定位回放");
    expect(markup).toContain("00:00 - 01:00");
    expect(markup).toContain("23:00 - 次日 00:00");
    expect(markup).toContain("回放范围");
  });
});
