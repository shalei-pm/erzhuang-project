// @vitest-environment jsdom
import { describe, expect, it, vi } from "vitest";
import { appHomePath, navigateAppHome } from "./url-utils";

describe("appHomePath", () => {
  it("returns the configured application home with a trailing slash", () => {
    expect(appHomePath()).toBe("/erzhuang-project/");
    expect(appHomePath("/erzhuang/")).toBe("/erzhuang/");
  });
});

describe("navigateAppHome", () => {
  it("switches to the home path without a document reload", () => {
    const onPopState = vi.fn();
    window.addEventListener("popstate", onPopState);
    navigateAppHome();
    expect(window.location.pathname).toBe("/erzhuang-project/");
    expect(onPopState).toHaveBeenCalledTimes(1);
    window.removeEventListener("popstate", onPopState);
  });
});
