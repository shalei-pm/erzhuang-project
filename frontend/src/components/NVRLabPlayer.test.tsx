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

  it("does not render playback transport diagnostics beside the video", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabPlayer, {
        session: { mode: "playback", url: "wss://example.test/session?token=private-token" },
        onRetry: () => undefined,
      }),
    );

    expect(markup).not.toContain("接收媒体包");
    expect(markup).not.toContain("WASM 输出帧");
    expect(markup).not.toContain("private-token");
  });

  it("renders the sound and local pause controls for a playable session", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabPlayer, {
        session: { mode: "live", url: "wss://example.test/session" },
        onRetry: () => undefined,
      }),
    );

    expect(markup).toContain('aria-label="播放"');
    expect(markup).toContain('aria-label="开启声音"');
  });
});
