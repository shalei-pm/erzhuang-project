import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { PlaybackDatePicker } from "./PlaybackDatePicker";

describe("PlaybackDatePicker", () => {
  it("renders the 2.x quick dates, replay trigger and locate action", () => {
    const markup = renderToStaticMarkup(
      createElement(PlaybackDatePicker, {
        value: "2026-08-26T10:18",
        onChange: () => undefined,
        onConfirm: () => undefined,
      }),
    );

    expect(markup).toContain("今天");
    expect(markup).toContain("昨天");
    expect(markup).toContain("前天");
    expect(markup).toContain("回放时间");
    expect(markup).toContain("定位回放");
  });
});
