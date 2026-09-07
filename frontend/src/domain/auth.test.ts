import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { watchSessionDeadline } from "./session-deadline";

import {
  authLogoutPath,
  claimIdleSessionTimeoutRedirect,
  claimSessionRedirect,
  isIdleSessionTimeout,
  authStateFromError,
  authEntryPath,
  sessionAuthMessage,
  shouldBlockBusinessData,
  subscribeSessionAuthRequired,
  reportSessionAuthError,
  readSessionStorage,
  removeSessionStorage,
  safeSessionStorage,
  writeSessionStorage,
  type AuthState,
} from "./auth";

describe("non-renewing session deadline checks", () => {
  const minute = 60_000;
  const initial = { idle_remaining_ms: 30 * minute, absolute_remaining_ms: 480 * minute };
  let visibility: EventTarget & { visibilityState: string };
  beforeEach(() => {
    vi.useFakeTimers();
    visibility = Object.assign(new EventTarget(), { visibilityState: "visible" });
    vi.stubGlobal("document", visibility);
  });
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); });

  it("makes no requests before the deadline and stops after an idle 401", async () => {
    const error = { status: 401, code: "session_idle_timeout" };
    const check = vi.fn().mockRejectedValue(error);
    const onError = vi.fn();
    const stop = watchSessionDeadline(initial, check, onError);
    await vi.advanceTimersByTimeAsync(30 * minute - 1);
    expect(check).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(check).toHaveBeenCalledTimes(1);
    expect(onError).toHaveBeenCalledWith(error);
    await vi.advanceTimersByTimeAsync(480 * minute);
    visibility.dispatchEvent(new Event("visibilitychange"));
    expect(check).toHaveBeenCalledTimes(1);
    stop();
  });

  it("rearms from another tab's remaining lifetime and honors the earlier absolute limit", async () => {
    const check = vi.fn().mockResolvedValueOnce({ idle_remaining_ms: 30 * minute, absolute_remaining_ms: 5 * minute })
      .mockRejectedValueOnce({ status: 401, code: "session_absolute_timeout" });
    const onError = vi.fn();
    watchSessionDeadline(initial, check, onError);
    await vi.advanceTimersByTimeAsync(30 * minute);
    expect(check).toHaveBeenCalledTimes(1);
    expect(onError).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(5 * minute - 1);
    expect(check).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(onError).toHaveBeenCalledWith({ status: 401, code: "session_absolute_timeout" });
  });

  it("checks on visibility restoration, deduplicates in-flight checks and clears the old timer", async () => {
    let resolve!: (session: typeof initial) => void;
    const check = vi.fn(() => new Promise<typeof initial>((done) => { resolve = done; }));
    const stop = watchSessionDeadline(initial, check, vi.fn());
    visibility.visibilityState = "hidden";
    visibility.dispatchEvent(new Event("visibilitychange"));
    expect(check).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(10 * minute);
    visibility.visibilityState = "visible";
    visibility.dispatchEvent(new Event("visibilitychange"));
    visibility.dispatchEvent(new Event("visibilitychange"));
    expect(check).toHaveBeenCalledTimes(1);
    resolve(initial);
    await vi.advanceTimersByTimeAsync(20 * minute);
    expect(check).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(10 * minute);
    expect(check).toHaveBeenCalledTimes(2);
    stop();
  });

  it.each([new Error("network failed"), { status: 503 }])("blocks on status failure without retrying or renewing: %j", async (error) => {
    const check = vi.fn().mockRejectedValue(error);
    const onError = vi.fn();
    watchSessionDeadline({ ...initial, absolute_remaining_ms: minute }, check, onError);
    await vi.advanceTimersByTimeAsync(minute);
    expect(onError).toHaveBeenCalledWith(error);
    expect(shouldBlockBusinessData(authStateFromError(error))).toBe(true);
    await vi.advanceTimersByTimeAsync(60 * minute);
    expect(check).toHaveBeenCalledTimes(1);
  });

  it("discards a late successful status response after cleanup", async () => {
    let resolve!: (session: typeof initial) => void;
    const check = vi.fn(() => new Promise<typeof initial>((done) => { resolve = done; }));
    const stop = watchSessionDeadline(initial, check, vi.fn());
    await vi.advanceTimersByTimeAsync(30 * minute);
    stop();
    resolve(initial);
    await vi.advanceTimersByTimeAsync(60 * minute);
    visibility.dispatchEvent(new Event("visibilitychange"));
    expect(check).toHaveBeenCalledTimes(1);
  });

  it("fails closed when the non-renewing status request hangs", async () => {
    const check = vi.fn(() => new Promise<typeof initial>(() => undefined));
    const onError = vi.fn();
    watchSessionDeadline({ idle_remaining_ms: 1, absolute_remaining_ms: minute }, check, onError);
    await vi.advanceTimersByTimeAsync(1);
    expect(check).toHaveBeenCalledOnce();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(onError).toHaveBeenCalledOnce();
    expect(shouldBlockBusinessData(authStateFromError(onError.mock.calls[0][0]))).toBe(true);
  });

  it("blocks malformed status metadata rather than spinning on zero deadlines", async () => {
    const onError = vi.fn();
    const check = vi.fn().mockResolvedValue({ idle_remaining_ms: 0, absolute_remaining_ms: 1 });
    watchSessionDeadline(initial, check, onError);
    await vi.advanceTimersByTimeAsync(30 * minute);
    expect(onError).toHaveBeenCalledOnce();
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe("idle session auth helpers", () => {
  it.each(["session_idle_timeout", "session_absolute_timeout", "session_reauthentication_required"])("treats %s as a joint logout, preserving its reason", (code) => {
    const error = { status: 401, code };
    expect(isIdleSessionTimeout(error)).toBe(true);
    expect(isIdleSessionTimeout({ ...error, status: 403 })).toBe(false);
    const auth = authStateFromError(error);
    expect(auth).toMatchObject({ enabled: true, authenticated: false, code });
    expect(authEntryPath(auth, "lite.sy.soyoung.com")).toContain("logouttogether");
    expect(sessionAuthMessage(code)).toContain("登录");
  });

  it("routes initial login to the backend callback, never the company home or joint logout", () => {
    const error = { status: 401, code: "session_login_required", login_url: "/custom/callback?return_to=h5" };
    expect(isIdleSessionTimeout(error)).toBe(false);
    const auth = authStateFromError(error);
    expect(authEntryPath(auth, "lite.sy.soyoung.com")).toBe(error.login_url);
    expect(authEntryPath({ ...auth, login_url: undefined }, "lite.sy.soyoung.com")).toBe("/erzhuang-project/_/auth/callback");
  });

  it.each([new Error("network failed"), { status: 500 }, { status: 403 }])("fails closed on auth check failure: %j", (error) => {
    const auth = authStateFromError(error);
    expect(auth.authenticated).toBe(false);
    expect(shouldBlockBusinessData(auth)).toBe(true);
  });

  it("blocks unresolved or unauthenticated states even with SSO disabled", () => {
    expect(shouldBlockBusinessData(null)).toBe(true);
    expect(shouldBlockBusinessData({ enabled: false, authenticated: false })).toBe(true);
    expect(shouldBlockBusinessData({ enabled: false, authenticated: true })).toBe(false);
  });

  it("notifies global consumers only for recognized 401s and cleans up subscriptions", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeSessionAuthRequired(listener);
    const error = { status: 401, code: "session_login_required" };
    expect(reportSessionAuthError(error)).toBe(error);
    expect(listener).toHaveBeenCalledWith(error);
    reportSessionAuthError({ status: 403, code: "session_idle_timeout" });
    reportSessionAuthError({ status: 500 });
    reportSessionAuthError({ status: 401, code: "other" });
    expect(listener).toHaveBeenCalledTimes(1);
    unsubscribe();
    reportSessionAuthError(error);
    expect(listener).toHaveBeenCalledTimes(1);
  });

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

  it("claims any session redirect once and remains available without storage", () => {
    const values = new Map<string, string>();
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
    };

    expect(claimSessionRedirect("entry", storage)).toBe(true);
    expect(claimSessionRedirect("entry", storage)).toBe(false);
    expect(claimSessionRedirect("entry", null)).toBe(false);
  });

  it("does not claim the ordinary SSO entry redirect when storage operations fail", () => {
    const failingStorage = {
      getItem: () => {
        throw new Error("storage unavailable");
      },
      setItem: () => {
        throw new Error("storage unavailable");
      },
    };

    expect(claimSessionRedirect("entry", failingStorage)).toBe(false);
    expect(claimIdleSessionTimeoutRedirect(failingStorage)).toBe(true);
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
    expect(gatewayURL.origin).toBe("https://security.soyoung.com");
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
