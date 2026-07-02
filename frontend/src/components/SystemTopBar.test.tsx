import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { SystemTopBar } from "./SystemTopBar";
import type { AuthState } from "../domain/auth";

describe("SystemTopBar", () => {
  it("shows display name and never renders enterprise email", () => {
    const auth: AuthState = {
      enabled: true,
      authenticated: true,
      user: {
        email: "someone@example.com",
        username: "zhangsan",
        display_name: "张三",
        role: "admin",
      },
    };

    const markup = renderToStaticMarkup(createElement(SystemTopBar, { auth, onLogout: () => undefined }));

    expect(markup).toContain("张三");
    expect(markup).not.toContain("someone@example.com");
  });

  it("renders username fallback, back action, and logout loading state", () => {
    const auth: AuthState = {
      enabled: true,
      authenticated: true,
      user: {
        email: "fallback@example.com",
        username: "lisi",
        display_name: "",
        role: "admin",
      },
    };

    const markup = renderToStaticMarkup(
      createElement(SystemTopBar, {
        backAction: { label: "返回列表", onClick: () => undefined },
        auth,
        loggingOut: true,
        onLogout: () => undefined,
      }),
    );

    expect(markup).toContain("返回列表");
    expect(markup).toContain("lisi");
    expect(markup).toContain("退出中...");
    expect(markup).not.toContain("fallback@example.com");
  });

  it("places account actions between display name and logout", () => {
    const auth: AuthState = {
      enabled: true,
      authenticated: true,
      user: {
        email: "admin@example.com",
        username: "admin",
        display_name: "管理员",
        role: "admin",
      },
    };

    const markup = renderToStaticMarkup(
      createElement(SystemTopBar, {
        auth,
        onLogout: () => undefined,
        rightExtra: createElement("button", null, "系统设置"),
      }),
    );

    expect(markup.indexOf("管理员")).toBeLessThan(markup.indexOf("系统设置"));
    expect(markup.indexOf("系统设置")).toBeLessThan(markup.indexOf("退出登录"));
  });
});
