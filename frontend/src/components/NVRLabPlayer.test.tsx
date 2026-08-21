import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NVRLabPlayer } from "./NVRLabPlayer";

describe("NVRLabPlayer", () => {
  it("does not render the signed stream URL into the page", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabPlayer, {
        session: { mode: "playback", url: "wss://example.test/session?token=private-token" },
        onRetry: () => undefined,
      }),
    );

    expect(markup).toContain("录像");
    expect(markup).not.toContain("private-token");
  });
});
