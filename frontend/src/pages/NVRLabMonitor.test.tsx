import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NVRMonitorThumbnail } from "./NVRLabMonitor";

describe("NVRMonitorThumbnail", () => {
  it("renders a verified legacy thumbnail when one is available", () => {
    const html = renderToStaticMarkup(createElement(NVRMonitorThumbnail, { thumbnailURL: "/api/store-space-resource-view/stores/10001/cameras/111/snapshot" }));

    expect(html).toContain('src="/api/store-space-resource-view/stores/10001/cameras/111/snapshot"');
    expect(html).toContain('alt="摄像头最近截图"');
  });

  it("renders the neutral placeholder when no safe thumbnail is available", () => {
    const html = renderToStaticMarkup(createElement(NVRMonitorThumbnail, {}));

    expect(html).toContain("h5-camera-placeholder");
    expect(html).not.toContain("<img");
  });
});
