import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NVRLabHourlyPlaybackPicker } from "../pages/NVRLabCamera";

describe("NVRLabHourlyPlaybackPicker", () => {
  it("renders a date-linked hourly grid without a separate locate action", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabHourlyPlaybackPicker, {
        startAt: "2026-08-26T10:00",
        onStartAtChange: () => undefined,
        onPlay: () => undefined,
        now: new Date("2026-08-26T10:18:00"),
      }),
    );

    expect(markup).toContain("回放日期");
    expect(markup).not.toContain("h5-date-confirm");
    expect(markup).toContain("00:00 - 01:00");
    expect(markup).toContain("23:00 - 次日 00:00");
    expect(markup).toContain("disabled=\"\"");
  });
});
