import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NVRMonitorThumbnail } from "./NVRLabMonitor";
import { nvrLabThumbnailURL } from "../api-nvr-lab";
import { NVRLabMonitor } from "./NVRLabMonitor";

describe("NVRLabMonitor top bar", () => {
  it("offers a home action on the monitor list", () => {
    const html = renderToStaticMarkup(
      createElement(NVRLabMonitor, {
        externalOrgId: "10001",
        auth: null,
        loggingOut: false,
        authMessage: "",
        onLogout: () => undefined,
        onAuthRequired: () => undefined,
        onOpenCamera: () => undefined,
        onSelectStore: () => undefined,
      }),
    );
    expect(html).toContain('aria-label="回首页"');
  });
});

describe("NVRMonitorThumbnail", () => {
  it("keeps NVR thumbnail requests under the configured application base path", () => {
    expect(nvrLabThumbnailURL("/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot")).toBe("/erzhuang-project/api/h5/nvr-monitor/orgs/10001/cameras/111/snapshot");
  });

  it("renders a verified legacy thumbnail when one is available", () => {
    const html = renderToStaticMarkup(createElement(NVRMonitorThumbnail, { thumbnailURL: "/api/store-space-resource-view/stores/10001/cameras/111/snapshot" }));

    expect(html).toContain('src="/erzhuang-project/api/store-space-resource-view/stores/10001/cameras/111/snapshot"');
    expect(html).toContain('alt="摄像头最近截图"');
  });

  it("renders the classified illustration when no safe thumbnail is available", () => {
    const html = renderToStaticMarkup(createElement(NVRMonitorThumbnail, { thumbnailKind: "treatment" }));

    expect(html).toContain('src="/erzhuang-project/camera-placeholders/treatment.png"');
    expect(html).toContain('alt="摄像头默认缩略图"');
    expect(html).toContain('class="is-camera-placeholder"');
  });
});
