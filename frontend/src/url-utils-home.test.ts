import { describe, expect, it } from "vitest";
import { appHomePath } from "./url-utils";

describe("appHomePath", () => {
  it("returns the configured application home with a trailing slash", () => {
    expect(appHomePath()).toBe("/erzhuang-project/");
    expect(appHomePath("/erzhuang/")).toBe("/erzhuang/");
  });
});
