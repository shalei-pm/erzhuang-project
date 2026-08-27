import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { NVRLabPlayer, nvrLabFirstFrameTimeoutMessage } from "./NVRLabPlayer";
import { NVRLabCamera } from "../pages/NVRLabCamera";

describe("NVRLabPlayer", () => {
	it("classifies a connected stream that has not delivered any media packets", () => {
		expect(nvrLabFirstFrameTimeoutMessage({ receivedPackets: 0, decoderInputFrames: 0, renderedFrames: 0 })).toBe(
			"视频流已连接，但未收到摄像头媒体数据，请稍后重试",
		);
	});

	it("classifies media that the live player cannot turn into a video frame", () => {
		expect(nvrLabFirstFrameTimeoutMessage({ receivedPackets: 12, rtpPackets: 0, videoPayloadPackets: 0, vpsPackets: 0, spsPackets: 0, ppsPackets: 0, keyFrameNALUnits: 0, decoderInputFrames: 0, renderedFrames: 0 })).toBe(
			"视频流已收到，但数据格式不是有效 RTP，请确认工控机转发协议",
		);
	});

	it("identifies a live stream that has RTP video packets but lacks H.265 parameter sets", () => {
		expect(nvrLabFirstFrameTimeoutMessage({ receivedPackets: 12, rtpPackets: 12, videoPayloadPackets: 12, vpsPackets: 0, spsPackets: 0, ppsPackets: 0, keyFrameNALUnits: 0, decoderInputFrames: 0, renderedFrames: 0 })).toBe(
			"视频流已收到 PT=96 视频包，但缺少 H.265 解码参数集，请确认该通道编码",
		);
	});

	it("identifies a configured H.265 stream that has not delivered a key frame", () => {
		expect(nvrLabFirstFrameTimeoutMessage({ receivedPackets: 12, rtpPackets: 12, videoPayloadPackets: 12, vpsPackets: 1, spsPackets: 1, ppsPackets: 1, keyFrameNALUnits: 0, decoderInputFrames: 0, renderedFrames: 0 })).toBe(
			"视频流已收到，但未收到 H.265 关键帧，请确认工控机关键帧转发",
		);
	});

	it("classifies decoder input that cannot render in the current browser", () => {
		expect(nvrLabFirstFrameTimeoutMessage({ receivedPackets: 12, rtpPackets: 12, videoPayloadPackets: 12, vpsPackets: 1, spsPackets: 1, ppsPackets: 1, keyFrameNALUnits: 1, decoderInputFrames: 4, renderedFrames: 0 })).toBe(
			"视频流已收到，但当前浏览器无法解码该摄像头画面",
		);
	});

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

  it("only enables the one-shot canvas capture when explicitly requested", () => {
    const markup = renderToStaticMarkup(
      createElement(NVRLabPlayer, {
        session: { mode: "live", url: "wss://example.test/session" },
        captureOnFirstFrame: true,
        onSnapshotCapture: () => undefined,
        onRetry: () => undefined,
      }),
    );

    expect(markup).toContain('aria-label="NVR 视频画面"');
    expect(markup).not.toContain("snapshot_backfill");
  });
});
