import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { H5StoreSwitcher } from "./H5StoreSwitcher";

describe("H5StoreSwitcher", () => {
  it("renders the current store as a dropdown trigger", () => {
    const markup = renderToStaticMarkup(
      createElement(H5StoreSwitcher, {
        currentExternalOrgId: "10030",
        currentStoreName: "北京保利实验室门店",
        currentCity: "北京",
        onSelectStore: () => undefined,
      }),
    );

    expect(markup).toContain('aria-label="切换门店"');
    expect(markup).toContain("北京保利实验室门店");
    expect(markup).toContain("北京");
    expect(markup).toContain("h5-store-dropdown");
    expect(markup).toContain("h5-store-trigger");
    expect(markup).toContain("h5-store-trigger-title-row");
    expect(markup).toContain("h5-store-trigger-chevron-icon");
    expect(markup).toMatch(
      /h5-store-trigger-title-row[\s\S]*北京保利实验室门店[\s\S]*h5-store-trigger-chevron[\s\S]*h5-store-trigger-city/,
    );
    expect(markup).not.toContain("▾");
    expect(markup).not.toContain("h5-store-switcher-current");
  });
});
