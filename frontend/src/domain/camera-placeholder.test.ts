import { describe, expect, it } from "vitest";
import { cameraPlaceholderURL, legacyCameraThumbnailKind } from "./camera-placeholder";

describe("cameraPlaceholderURL", () => {
  it("uses the application base path and falls back to the unassigned illustration", () => {
    expect(cameraPlaceholderURL("consultation")).toBe("/erzhuang-project/camera-placeholders/consultation.png");
    expect(cameraPlaceholderURL("unknown")).toBe("/erzhuang-project/camera-placeholders/unassigned.png");
  });
});

describe("legacyCameraThumbnailKind", () => {
  it("maps the legacy monitor metadata into the shared placeholder set", () => {
    expect(legacyCameraThumbnailKind({ areaType: "consultation" })).toBe("consultation");
    expect(legacyCameraThumbnailKind({ areaType: "beauty" })).toBe("treatment");
    expect(legacyCameraThumbnailKind({ sceneType: "front_desk" })).toBe("reception");
    expect(legacyCameraThumbnailKind({ sceneType: "waiting_area" })).toBe("waiting");
    expect(legacyCameraThumbnailKind({ sceneType: "passage" })).toBe("corridor");
    expect(legacyCameraThumbnailKind({ category: "other" })).toBe("utility");
    expect(legacyCameraThumbnailKind({})).toBe("unassigned");
  });
});
