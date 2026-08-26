import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { NVRLabPlayer } from "./NVRLabPlayer";
import { NVRLabCamera } from "../pages/NVRLabCamera";

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

  it("renders the 2.x playback progress slider for an active NVR playback window", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabPlayer, {
        session: { mode: "playback", url: "wss://example.test/session" },
        playbackSegment: { start_time: 1_000, end_time: 1_060 },
        playbackCursorUnix: 1_020,
        onSeekPlayback: () => undefined,
        onRetry: () => undefined,
      }),
    );

    expect(markup).toContain('aria-label="片段内定位"');
    expect(markup).toContain('type="range"');
    expect(markup).toContain('value="20"');
  });

  it("renders the live and playback switcher as the shared page bottom bar", () => {
    vi.stubGlobal("window", { location: { search: "" } });
    const markup = renderToStaticMarkup(
      createElement(NVRLabCamera, {
		externalOrgId: "10001",
        cameraId: 111,
        auth: null,
        loggingOut: false,
        authMessage: "",
        onLogout: () => undefined,
        onAuthRequired: () => undefined,
        onBack: () => undefined,
      }),
    );

    expect(markup).toContain('class="h5-bottom-tabs"');
    expect(markup).toContain('aria-label="播放模式"');
    expect(markup).not.toContain('class="nvr-lab-control-tabs"');
    expect(markup).toContain("实时视频");
    expect(markup).toContain("录像");
    vi.unstubAllGlobals();
  });
});
