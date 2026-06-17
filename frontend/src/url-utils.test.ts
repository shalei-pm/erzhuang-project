import { defaultApiBase, displayImageUrl, storedImagePath } from "./url-utils.js";

const mockPlanImage = "/assets/mock-plan.png";

assertEqual(
  displayImageUrl("/api/design-plan/stores/12/preview", {
    apiBase: "/erzhuang-project/api/design-plan",
    mockPlanImage,
  }),
  "/erzhuang-project/api/design-plan/stores/12/preview",
);

assertEqual(
  displayImageUrl("/api/store-space/channel-snapshots/00000000000000000000000000000001.jpg", {
    apiBase: "/erzhuang-project/api/store-space",
    mockPlanImage,
  }),
  "/erzhuang-project/api/store-space/channel-snapshots/00000000000000000000000000000001.jpg",
);

assertEqual(
  displayImageUrl("uploads/tmp_123/thumbnail.png", {
    apiBase: "/erzhuang/api/design-plan",
    mockPlanImage,
  }),
  "/erzhuang/api/design-plan/uploads/tmp_123/thumbnail",
);

assertEqual(
  storedImagePath("/erzhuang-project/api/design-plan/uploads/tmp_123/preview", "fallback.png"),
  "uploads/tmp_123/preview.png",
);

Object.defineProperty(globalThis, "window", {
  value: { location: { pathname: "/erzhuang-project/stores/1" } },
  configurable: true,
});

assertEqual(defaultApiBase("store-space", "/erzhuang/"), "/erzhuang-project/api/store-space");

console.log("url-utils tests passed");

function assertEqual(actual: string, expected: string) {
  if (actual !== expected) {
    throw new Error(`expected ${expected}, got ${actual}`);
  }
}
