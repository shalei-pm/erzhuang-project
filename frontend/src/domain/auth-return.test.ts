import { afterEach, expect, it, vi } from "vitest";
import { authEntryPath, authLogoutPath, consumeAuthReturnPath, safeAuthReturnPath } from "./auth";

afterEach(() => vi.unstubAllGlobals());
const home = "/erzhuang-project/";
const target = `${home}h5/orgs/10001/monitor/cameras/111?tab=playback#time`;
function browser(path: string) {
  const url = new URL(path, "https://lite.sy.soyoung.com");
  const replace = vi.fn();
  vi.stubGlobal("window", { location: { href: url.href, hostname: url.hostname, pathname: url.pathname, search: url.search, hash: url.hash, replace }, history: { state: null, replaceState: vi.fn() } });
  return replace;
}
it("carries deep links through entry and callback without changing SSO", () => {
  browser(target);
  const entry = authEntryPath(null);
  expect(new URL(entry, "https://lite.sy.soyoung.com").searchParams.get("return_to")).toBe(target);
  browser(entry);
  const callback = authEntryPath({enabled: true, authenticated: false, code: "session_login_required"});
  expect(new URL(callback, "https://lite.sy.soyoung.com").searchParams.get("return_to")).toBe(target);
});
it("returns manual logout to the current page and explicit home to home", () => {
  browser(target);
  const logout = new URL(authLogoutPath(), "https://lite.sy.soyoung.com");
  expect(new URL(logout.searchParams.get("redirect")!).searchParams.get("from_uri")).toBe(`https://lite.sy.soyoung.com${target}`);
  browser(home);
  expect(authEntryPath(null)).toBe(home);
});
it("consumes the return target after successful authentication", () => {
  const replace = browser(`${home}?return_to=${encodeURIComponent(target)}`);
  expect(consumeAuthReturnPath()).toBe(true);
  expect(replace).toHaveBeenCalledWith(target);
});
it("rejects external, escaped and authentication endpoints", () => {
  for (const value of ["https://evil.example", "//evil.example", `${home}../other`, `${home}%2e%2e/other`, `${home}%5cevil`, `${home}%252e%252e/other`, `${home}api/auth/logout`, `${home}_/auth/callback`, `${home}logout`]) expect(safeAuthReturnPath(value)).toBe(home);
  expect(safeAuthReturnPath(target)).toBe(target);
});
