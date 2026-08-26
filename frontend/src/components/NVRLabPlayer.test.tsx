import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NVRLabPlayer } from "./NVRLabPlayer";

describe("NVRLabPlayer", () => {
  it("does not render the signed stream URL into the page", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabPlayer, {
        session: { mode: "playback", url: "wss://example.test/session?token=private-token" },
        mode: "playback",
        onModeChange: () => undefined,
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
        mode: "playback",
        onModeChange: () => undefined,
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
        mode: "live",
        onModeChange: () => undefined,
        onRetry: () => undefined,
      }),
    );

    expect(markup).toContain('aria-label="播放"');
    expect(markup).toContain('aria-label="开启声音"');
  });

  it("renders the live and playback switcher inside the player controls", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabPlayer, {
        session: { mode: "live", url: "wss://example.test/session" },
        mode: "live",
        onModeChange: () => undefined,
        onRetry: () => undefined,
      }),
    );

    expect(markup).toContain('class="nvr-lab-control-tabs"');
    expect(markup).toContain('aria-label="视频模式"');
    expect(markup).toContain('aria-pressed="true"');
    expect(markup).toContain("实时视频");
    expect(markup).toContain("录像");
  });
});
