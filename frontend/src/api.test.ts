import { describe, expect, it } from "vitest";

import { __testing } from "./api";
import { isH5FirstFrameEvent } from "./domain/h5-player-diagnostics";
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

describe("H5 monitor trial entry", () => {
  it("only allows the pilot org 10030 to open H5 monitor", () => {
    expect(canOpenH5Monitor({ externalOrgId: "10030" })).toBe(true);
    expect(canOpenH5Monitor({ externalOrgId: "010030" })).toBe(false);
    expect(canOpenH5Monitor({ externalOrgId: "10031" })).toBe(false);
    expect(canOpenH5Monitor({ externalOrgId: "" })).toBe(false);
  });

  it("builds the H5 monitor path with the current project base", () => {
    expect(h5MonitorPath("10030")).toBe("/erzhuang-project/h5/orgs/10030/monitor");
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
});
