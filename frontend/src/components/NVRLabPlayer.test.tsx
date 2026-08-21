import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NVRLabPlaybackDiagnostics, NVRLabPlayer } from "./NVRLabPlayer";

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

  it("renders safe playback transport diagnostics without rendering the signed URL", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabPlaybackDiagnostics, {
        diagnostics: {
          receivedPackets: 12,
          wasmRuntimeReady: true,
          wasmReady: true,
          wasmOutputInit: 1,
          wasmOutputFrames: 8,
          decoderInputFrames: 7,
          renderedFrames: 6,
          closeCode: null,
        },
      }),
    );

    expect(markup).toContain("接收媒体包");
    expect(markup).toContain("12");
    expect(markup).toContain("WASM 输出帧");
    expect(markup).not.toContain("private-token");
  });
});
