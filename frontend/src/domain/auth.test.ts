import { describe, expect, it } from "vitest";

import {
  authLogoutPath,
  claimIdleSessionTimeoutRedirect,
  isIdleSessionTimeout,
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
  });
});
