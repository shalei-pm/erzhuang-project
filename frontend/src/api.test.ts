import { describe, expect, it } from "vitest";

import { __testing } from "./api";
import { h5CameraColumnCount, h5ChannelDisplayText, h5InitialVisibleCount, h5NextVisibleCount } from "./domain/h5-channel-display";
import {
  h5DecodePathForEnvironment,
  isH5FirstFrameEvent,
  shouldFallbackH5MSEToSoftDecode,
  shouldUseH5SoftDecode,
} from "./domain/h5-player-diagnostics";
import {
  clampSegmentOffset,
  dataUrlToFile,
  estimatePlaybackUnixAt,
  nextRecordSegmentIndex,
  playbackUnixFromPlayerTime,
  segmentDurationSeconds,
  segmentOffsetToUnix,
  shouldFallbackToInlineFullscreen,
  shouldShowSegmentSlider,
} from "./domain/h5-playback";
import {
  authCompanyEntryPath,
  authLoginPath,
  authUserDisplayName,
  authLogoutPath,
  shouldBlockBusinessData,
  shouldShowForbiddenAccess,
  shouldShowLoginWelcome,
  shouldShowLogoutEntry,
  shouldSkipLocalLogoutBeforeRedirect,
} from "./domain/auth";
import { canOpenH5Monitor, h5MonitorPath } from "./domain/store-detail-navigation";

describe("design plan API path helpers", () => {
  it("adds the project base path to backend image URLs", () => {
    expect(__testing.toDisplayImageUrl("/api/design-plan/stores/12/thumbnail")).toBe(
      "/erzhuang-project/api/design-plan/stores/12/thumbnail",
    );
  });

  it("adds the project base path to stored upload image paths", () => {
    expect(__testing.toDisplayImageUrl("uploads/upload-1/preview.png")).toBe(
      "/erzhuang-project/api/design-plan/uploads/upload-1/preview",
    );
  });

  it("stores upload image paths from the current project base path", () => {
    expect(
      __testing.toStoredPath(
        "/erzhuang-project/api/design-plan/uploads/upload-1/preview",
        "mock/fallback.png",
      ),
    ).toBe("uploads/upload-1/preview.png");
  });

  it("keeps compatibility with the legacy project base path", () => {
    expect(__testing.toStoredPath("/erzhuang/api/design-plan/uploads/upload-1/thumbnail", "mock/fallback.png")).toBe(
      "uploads/upload-1/thumbnail.png",
    );
  });
});

describe("store list summary", () => {
  it("summarizes all filtered stores instead of the current page only", () => {
    const stores = [
      { id: 1, treatmentCount: 1, consultationCount: 0, beautyCount: 0 },
      { id: 2, treatmentCount: 0, consultationCount: 2, beautyCount: 1 },
    ];

    expect(__testing.summarizeStoreSummaries(stores as Parameters<typeof __testing.summarizeStoreSummaries>[0])).toEqual({
      storeCount: 2,
      treatmentCount: 1,
      consultationCount: 2,
      beautyCount: 1,
    });
  });
});

describe("auth helpers", () => {
  it("keeps the existing app open while sso is disabled", () => {
    expect(shouldShowLoginWelcome({ enabled: false, authenticated: true })).toBe(false);
  });

  it("shows the sso welcome page only when sso is enabled and unauthenticated", () => {
    expect(shouldShowLoginWelcome({ enabled: true, authenticated: false })).toBe(true);
    expect(shouldShowLoginWelcome({ enabled: true, authenticated: true })).toBe(false);
  });

  it("blocks business data and shows a forbidden access state for unauthorized sso users", () => {
    const forbiddenAuth = { enabled: true, authenticated: false, forbidden: true };
    expect(shouldShowForbiddenAccess(forbiddenAuth)).toBe(true);
    expect(shouldBlockBusinessData(forbiddenAuth)).toBe(true);
    expect(shouldShowLoginWelcome(forbiddenAuth)).toBe(false);
    expect(shouldBlockBusinessData({ enabled: true, authenticated: true })).toBe(false);
  });

  it("builds the login path with the project base by default", () => {
    expect(authLoginPath()).toBe("/erzhuang-project/_/auth/callback");
    expect(authLoginPath("/custom/auth/login")).toBe("/custom/auth/login");
  });

  it("uses the project entry path on the company domain so apisix keeps the full return state", () => {
    expect(authCompanyEntryPath("lite.sy.soyoung.com")).toBe("/erzhuang-project/");
    expect(authCompanyEntryPath("127.0.0.1")).toBe("");
  });

  it("builds the apisix logout path with the project base", () => {
    expect(authLogoutPath()).toBe("/erzhuang-project/logout");
    expect(authLogoutPath("lite.sy.soyoung.com")).toBe(
      "/erzhuang-project/logout?redirect=https%3A%2F%2Fsecurity-test.sy.soyoung.com%2Fapi%2Fg%2Fsso%2Flogouttogether%3Ffrom_host%3Dlite.sy.soyoung.com%26from_uri%3Dhttps%253A%252F%252Flite.sy.soyoung.com%252Ferzhuang-project%252F",
    );
  });

  it("uses a top-level same-origin logout hop on the company domain", () => {
    expect(shouldSkipLocalLogoutBeforeRedirect("lite.sy.soyoung.com")).toBe(true);
    expect(shouldSkipLocalLogoutBeforeRedirect("127.0.0.1")).toBe(false);
  });

  it("shows logout on the company sso domain even while backend auth remains in compatibility mode", () => {
    expect(shouldShowLogoutEntry({ enabled: false, authenticated: true }, "lite.sy.soyoung.com")).toBe(true);
    expect(shouldShowLogoutEntry({ enabled: false, authenticated: true }, "127.0.0.1")).toBe(false);
    expect(shouldShowLogoutEntry({ enabled: true, authenticated: true }, "127.0.0.1")).toBe(true);
  });

  it("does not expose enterprise email as the display name", () => {
    expect(authUserDisplayName({ display_name: "沙磊", username: "shalei" })).toBe("沙磊");
    expect(authUserDisplayName({ display_name: "", username: "shalei" })).toBe("shalei");
    expect(authUserDisplayName({ display_name: "", username: "" })).toBe("已登录");
  });
});

describe("H5 monitor trial entry", () => {
  it("allows stores with an external org id to open H5 monitor", () => {
    expect(canOpenH5Monitor({ externalOrgId: "10030" })).toBe(true);
    expect(canOpenH5Monitor({ externalOrgId: "10047" })).toBe(true);
    expect(canOpenH5Monitor({ externalOrgId: "010030" })).toBe(true);
    expect(canOpenH5Monitor({ externalOrgId: "10031" })).toBe(true);
    expect(canOpenH5Monitor({ externalOrgId: " 10031 " })).toBe(true);
    expect(canOpenH5Monitor({ externalOrgId: "" })).toBe(false);
    expect(canOpenH5Monitor({ externalOrgId: "   " })).toBe(false);
  });

  it("builds the H5 monitor path with the current project base", () => {
    expect(h5MonitorPath("10030")).toBe("/erzhuang-project/h5/orgs/10030/monitor");
  });
});

describe("H5 monitor channel display text", () => {
  it("uses area and number as the primary title and channel number as the subtitle", () => {
    expect(
      h5ChannelDisplayText({
        id: 1,
        channel_no: 12,
        channel_name: "通道12",
        category: "treatment",
        area_type: "treatment",
        scene_type: "",
        area_number: 1,
        area_note: "",
        thumbnail_url: "",
      }),
    ).toEqual({ title: "治疗室1号", subtitle: "通道12" });
  });

  it("uses area note as the business suffix before falling back to raw channel name", () => {
    expect(
      h5ChannelDisplayText({
        id: 2,
        channel_no: 17,
        channel_name: "通道17",
        category: "treatment",
        area_type: "treatment",
        scene_type: "",
        area_number: 0,
        area_note: "401号",
        thumbnail_url: "",
      }).title,
    ).toBe("治疗室401号");

    expect(
      h5ChannelDisplayText({
        id: 3,
        channel_no: 16,
        channel_name: "通道16",
        category: "front_waiting",
        area_type: "",
        scene_type: "",
        area_number: 0,
        area_note: "护士站",
        thumbnail_url: "",
      }),
    ).toEqual({ title: "护士站", subtitle: "通道16" });
  });

  it("loads camera bubbles in complete rows based on the current grid columns", () => {
    expect(h5CameraColumnCount(1360, 1440)).toBe(7);
    expect(h5InitialVisibleCount(1360, 1440)).toBe(28);
    expect(h5NextVisibleCount({ containerWidth: 1360, viewportWidth: 1440, visibleCount: 28, totalCount: 36 })).toBe(36);

    expect(h5CameraColumnCount(390, 390)).toBe(3);
    expect(h5InitialVisibleCount(390, 390)).toBe(12);
    expect(h5NextVisibleCount({ containerWidth: 390, viewportWidth: 390, visibleCount: 12, totalCount: 26 })).toBe(18);
  });
});

describe("H5 player diagnostics", () => {
  it("only treats real render events as mobile wasm first frame", () => {
    expect(isH5FirstFrameEvent("loaded", "mobile-wasm")).toBe(false);
    expect(isH5FirstFrameEvent("playing", "mobile-wasm")).toBe(false);
    expect(isH5FirstFrameEvent("streamSuccess", "mobile-wasm")).toBe(false);
    expect(isH5FirstFrameEvent("videoInfo", "mobile-wasm")).toBe(false);
    expect(isH5FirstFrameEvent("videoFrame", "mobile-wasm")).toBe(true);
    expect(isH5FirstFrameEvent("firstFrameDisplay", "mobile-wasm")).toBe(true);
    expect(isH5FirstFrameEvent("playToRenderTimes", "mobile-wasm")).toBe(true);
  });

  it("keeps desktop mse first-frame detection compatible with existing events", () => {
    expect(isH5FirstFrameEvent("streamSuccess", "desktop-mse")).toBe(true);
    expect(isH5FirstFrameEvent("videoInfo", "desktop-mse")).toBe(true);
    expect(isH5FirstFrameEvent("loaded", "desktop-mse")).toBe(true);
    expect(isH5FirstFrameEvent("playing", "desktop-mse")).toBe(true);
  });

  it("keeps Windows desktop on MSE until the player reports unsupported H265", () => {
    const edgeWindowsUA =
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36 Edg/151.";
    expect(h5DecodePathForEnvironment(edgeWindowsUA, 0)).toBe("desktop-mse");
    expect(shouldUseH5SoftDecode("desktop-wasm")).toBe(true);
    expect(isH5FirstFrameEvent("loaded", "desktop-wasm")).toBe(false);
    expect(isH5FirstFrameEvent("videoFrame", "desktop-wasm")).toBe(true);
  });

  it("keeps non-Windows desktop on MSE and touch mobile on wasm", () => {
    const macUA =
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36";
    const iphoneUA =
      "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Lark";
    expect(h5DecodePathForEnvironment(macUA, 0)).toBe("desktop-mse");
    expect(shouldUseH5SoftDecode("desktop-mse")).toBe(false);
    expect(h5DecodePathForEnvironment(iphoneUA, 5)).toBe("mobile-wasm");
  });

  it("falls back from desktop MSE only for explicit H265 SourceBuffer unsupported errors", () => {
    const unsupportedH265 =
      "error:{\"errorTypes\":\"MediaError\",\"errorDetails\":\"MediaMSEError\",\"info\":{\"code\":9,\"msg\":\"Failed to execute 'addSourceBuffer' on 'MediaSource': The type provided ('video/mp4;codecs=hvc1.1.1.L63.B0') is unsupported.\"}}";
    expect(shouldFallbackH5MSEToSoftDecode(unsupportedH265, "desktop-mse")).toBe(true);
    expect(shouldFallbackH5MSEToSoftDecode(unsupportedH265, "desktop-wasm")).toBe(false);
    expect(shouldFallbackH5MSEToSoftDecode("stats:{\"fps\":0}", "desktop-mse")).toBe(false);
    expect(shouldFallbackH5MSEToSoftDecode("mseSourceBufferFull", "desktop-mse")).toBe(false);
  });
});

describe("H5 playback segment slider helpers", () => {
  it("calculates segment slider bounds and selected unix time", () => {
    expect(segmentDurationSeconds(100, 130)).toBe(30);
    expect(segmentDurationSeconds(130, 100)).toBe(0);
    expect(shouldShowSegmentSlider(100, 101)).toBe(false);
    expect(shouldShowSegmentSlider(100, 102)).toBe(true);
    expect(clampSegmentOffset(-5, 100, 130)).toBe(0);
    expect(clampSegmentOffset(35, 100, 130)).toBe(30);
    expect(segmentOffsetToUnix(100, 130, 12.8)).toBe(112);
  });

  it("estimates playback unix time from a session start and elapsed wall time", () => {
    expect(estimatePlaybackUnixAt(11_500, { startTime: 100, endTime: 130, startedAtMs: 1_000 })).toBe(110);
    expect(estimatePlaybackUnixAt(99_000, { startTime: 100, endTime: 130, startedAtMs: 1_000 })).toBe(129);
    expect(estimatePlaybackUnixAt(11_500, null)).toBe(null);
  });

  it("uses player current time when resuming playback from pause", () => {
    const session = { startTime: 100, endTime: 130, startedAtMs: 1_000 };
    expect(playbackUnixFromPlayerTime(12.8, session)).toBe(112);
    expect(playbackUnixFromPlayerTime(99, session)).toBe(129);
    expect(playbackUnixFromPlayerTime(Number.NaN, session)).toBe(null);
    expect(estimatePlaybackUnixAt(20_000, { ...session, pausedAtUnix: 115 })).toBe(115);
  });

  it("finds the next record segment for auto-advance", () => {
    const segments = [
      { start_time: 100, end_time: 130 },
      { start_time: 140, end_time: 160 },
      { start_time: 160, end_time: 190 },
    ];
    expect(nextRecordSegmentIndex(segments, segments[0])).toBe(1);
    expect(nextRecordSegmentIndex(segments, segments[2])).toBe(null);
    expect(nextRecordSegmentIndex(segments, { start_time: 101, end_time: 130 })).toBe(1);
    expect(nextRecordSegmentIndex(segments, null)).toBe(null);
  });

  it("converts screenshot data url into a shareable file", async () => {
    const file = await dataUrlToFile("data:image/png;base64,aGVsbG8=", "snapshot.png");
    expect(file.name).toBe("snapshot.png");
    expect(file.type).toBe("image/png");
    expect(file.size).toBe(5);
  });

  it("falls back to inline fullscreen on mobile browsers without fullscreen api", () => {
    expect(
      shouldFallbackToInlineFullscreen(
        { fullscreenEnabled: false },
        { maxTouchPoints: 5, userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS) Lark" },
      ),
    ).toBe(true);
    expect(
      shouldFallbackToInlineFullscreen(
        { fullscreenEnabled: true },
        { maxTouchPoints: 5, userAgent: "Mozilla/5.0 (Linux; Android 14; Mobile)" },
      ),
    ).toBe(false);
    expect(
      shouldFallbackToInlineFullscreen(
        { fullscreenEnabled: false },
        { maxTouchPoints: 0, userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X)" },
      ),
    ).toBe(false);
  });
});
