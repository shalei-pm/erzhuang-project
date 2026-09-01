import { describe, expect, it, vi } from "vitest";

import {
  authLogoutPath,
  claimIdleSessionTimeoutRedirect,
  isIdleSessionTimeout,
  readSessionStorage,
  removeSessionStorage,
  safeSessionStorage,
  writeSessionStorage,
  type AuthState,
} from "./auth";

describe("idle session auth helpers", () => {
  it("preserves the structured timeout code on auth state", () => {
    const auth: AuthState = {
      enabled: true,
      authenticated: false,
      code: "session_idle_timeout",
    };

    expect(auth.code).toBe("session_idle_timeout");
    expect(isIdleSessionTimeout({ status: 401, code: auth.code })).toBe(true);
    expect(isIdleSessionTimeout({ status: 403, code: "session_idle_timeout" })).toBe(false);
    expect(isIdleSessionTimeout({ status: 401, code: "" })).toBe(false);
  });

  it("claims timeout redirect once and uses the existing company logout path", () => {
    const values = new Map<string, string>();
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
    };

    expect(claimIdleSessionTimeoutRedirect(storage)).toBe(true);
    expect(claimIdleSessionTimeoutRedirect(storage)).toBe(false);
    expect(authLogoutPath("lite.sy.soyoung.com")).toContain("logouttogether");
  });

  it("continues the timeout logout when storage cannot be read or written", () => {
    const readErrorStorage = {
      getItem: () => {
        throw new Error("storage unavailable");
      },
      setItem: () => undefined,
    };
    const writeErrorStorage = {
      getItem: () => null,
      setItem: () => {
        throw new Error("storage unavailable");
      },
    };

    expect(claimIdleSessionTimeoutRedirect(readErrorStorage)).toBe(true);
    expect(claimIdleSessionTimeoutRedirect(writeErrorStorage)).toBe(true);
    expect(claimIdleSessionTimeoutRedirect(null)).toBe(true);
  });

  it("treats unavailable session storage as a non-blocking condition", () => {
    const failingStorage = {
      getItem: () => {
        throw new Error("storage unavailable");
      },
      setItem: () => {
        throw new Error("storage unavailable");
      },
      removeItem: () => {
        throw new Error("storage unavailable");
      },
    };

    expect(readSessionStorage("key", failingStorage)).toBeNull();
    expect(writeSessionStorage("key", "value", failingStorage)).toBe(false);
    expect(removeSessionStorage("key", failingStorage)).toBe(false);
  });

  it("returns no storage when the sessionStorage getter throws", () => {
    vi.stubGlobal("window", {
      get sessionStorage() {
        throw new Error("storage unavailable");
      },
    });

    expect(safeSessionStorage()).toBeNull();
    vi.unstubAllGlobals();
  });
});

describe("company SSO logout paths", () => {
  it("supports the formal domain with the safe gateway logout redirect", () => {
    const logoutURL = new URL(`https://lite.soyoung.com${authLogoutPath("lite.soyoung.com")}`);
    const gatewayURL = new URL(logoutURL.searchParams.get("redirect") ?? "");

    expect(logoutURL.pathname).toBe("/erzhuang-project/logout");
    expect(gatewayURL.origin).toBe("https://security-test.sy.soyoung.com");
    expect(gatewayURL.pathname).toBe("/api/g/sso/logouttogether");
    expect(gatewayURL.searchParams.get("from_host")).toBe("lite.soyoung.com");
    expect(gatewayURL.searchParams.get("from_uri")).toBe("http://lite.soyoung.com/erzhuang-project/");
  });

  it("normalizes the supported test domain without changing its HTTPS return URI", () => {
    const logoutURL = new URL(`https://lite.sy.soyoung.com${authLogoutPath("LITE.SY.SOYOUNG.COM.")}`);
    const gatewayURL = new URL(logoutURL.searchParams.get("redirect") ?? "");

    expect(gatewayURL.searchParams.get("from_host")).toBe("lite.sy.soyoung.com");
    expect(gatewayURL.searchParams.get("from_uri")).toBe("https://lite.sy.soyoung.com/erzhuang-project/");
  });
});
