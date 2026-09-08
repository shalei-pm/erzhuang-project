export type AuthUser = {
  email: string;
  username: string;
  display_name: string;
  open_id?: string;
  feishu_user_id?: string;
  phone?: string;
  login_way?: string;
  subject?: string;
  role: string;
};

export type SessionRemaining = {
  idle_remaining_ms: number;
  absolute_remaining_ms: number;
};

export type AuthState = {
  enabled: boolean;
  authenticated: boolean;
  code?: string;
  forbidden?: boolean;
  login_url?: string;
  user?: AuthUser;
  permissions?: string[];
  session?: SessionRemaining;
};

export type SessionStorageLike = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export const idleSessionTimeoutRedirectKey = "erzhuang:idle-session-timeout-redirected";
export const sessionLoginRedirectKey = "erzhuang:session-login-redirected";

const jointLogoutCodes = new Set(["session_idle_timeout", "session_absolute_timeout", "session_reauthentication_required"]);
const sessionAuthListeners = new Set<(error: unknown) => void>();

export function subscribeSessionAuthRequired(listener: (error: unknown) => void): () => void {
  sessionAuthListeners.add(listener);
  return () => { sessionAuthListeners.delete(listener); };
}

export function reportSessionAuthError<T>(error: T): T {
  if (isIdleSessionTimeout(error) || isSessionLoginRequired(error)) {
    sessionAuthListeners.forEach((listener) => listener(error));
  }
  return error;
}

export function isSessionLoginRequired(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const candidate = error as { status?: unknown; code?: unknown };
  return candidate.status === 401 && candidate.code === "session_login_required";
}

export function sessionAuthMessage(code?: string): string {
  switch (code) {
    case "session_idle_timeout": return "登录已因长时间未操作失效，请重新扫码登录。";
    case "session_absolute_timeout": return "本次登录已超过8小时，请重新扫码登录。";
    case "session_reauthentication_required": return "当前登录凭证已失效，请重新扫码登录。";
    case "session_login_required": return "需要完成登录验证后继续访问。";
    case "auth_check_failed": return "登录状态加载失败，请稍后重试。";
    default: return "";
  }
}

export function authStateFromError(error: unknown): AuthState {
  const candidate = error && typeof error === "object" ? error as { status?: unknown; code?: unknown; login_url?: unknown } : {};
  return {
    enabled: true,
    authenticated: false,
    forbidden: candidate.status === 403,
    code: candidate.status === 401 ? (typeof candidate.code === "string" ? candidate.code : undefined) : "auth_check_failed",
    login_url: typeof candidate.login_url === "string" ? candidate.login_url : undefined,
  };
}

export function authEntryPath(auth: AuthState | null, hostname = currentHostname()): string {
  if (isIdleSessionTimeout({ status: 401, code: auth?.code })) return authLogoutPath(hostname);
  if (auth?.code === "session_login_required") return authLoginPath(auth.login_url);
  return authCompanyEntryPath(hostname) || authLoginPath(auth?.login_url);
}

export function claimSessionRedirect(
  key: string,
  storage: Pick<Storage, "getItem" | "setItem"> | null,
  unavailableResult = false,
): boolean {
  if (!storage) return unavailableResult;
  try {
    if (storage.getItem(key) === "1") return false;
    storage.setItem(key, "1");
    return true;
  } catch {
    return unavailableResult;
  }
}

export function isIdleSessionTimeout(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const candidate = error as { status?: unknown; code?: unknown };
  // Compatibility entry point for existing child-page timeout handlers.
  return candidate.status === 401 && typeof candidate.code === "string" && jointLogoutCodes.has(candidate.code);
}

export function claimIdleSessionTimeoutRedirect(storage: Pick<Storage, "getItem" | "setItem"> | null): boolean {
  return claimSessionRedirect(idleSessionTimeoutRedirectKey, storage, true);
}

export function safeSessionStorage(): SessionStorageLike | null {
  try {
    return typeof window === "undefined" ? null : window.sessionStorage;
  } catch {
    return null;
  }
}

export function readSessionStorage(key: string, storage: Pick<SessionStorageLike, "getItem"> | null = safeSessionStorage()): string | null {
  if (!storage) return null;
  try {
    return storage.getItem(key);
  } catch {
    return null;
  }
}

export function writeSessionStorage(key: string, value: string, storage: Pick<SessionStorageLike, "setItem"> | null = safeSessionStorage()): boolean {
  if (!storage) return false;
  try {
    storage.setItem(key, value);
    return true;
  } catch {
    return false;
  }
}

export function removeSessionStorage(key: string, storage: Pick<SessionStorageLike, "removeItem"> | null = safeSessionStorage()): boolean {
  if (!storage) return false;
  try {
    storage.removeItem(key);
    return true;
  } catch {
    return false;
  }
}

export function shouldShowLoginWelcome(auth: Pick<AuthState, "enabled" | "authenticated" | "forbidden"> | null) {
  return Boolean(auth && !auth.authenticated && !auth.forbidden);
}

export function shouldShowForbiddenAccess(auth: Pick<AuthState, "forbidden"> | null) {
  return Boolean(auth?.forbidden);
}

export function shouldBlockBusinessData(auth: Pick<AuthState, "enabled" | "authenticated" | "forbidden"> | null) {
  return !auth?.authenticated || shouldShowForbiddenAccess(auth);
}

export function authLoginPath(loginUrl?: string) {
  return withAuthReturnPath(loginUrl || `${authBasePath()}/_/auth/callback`);
}

export function safeAuthReturnPath(value: string): string {
  const home = `${authBasePath()}/`;
  try {
    if (!value.startsWith(home) || /[\\\x00-\x20]/.test(value)) return home;
    const parsed = new URL(value, "https://return.invalid");
    const path = decodeURIComponent(parsed.pathname);
    if (!path.startsWith(home) || /[\\\x00-\x20]/.test(path) || path.includes("%")) return home;
    const relative = path.slice(home.length);
    if (["api", "_", "logout"].some((prefix) => relative === prefix || relative.startsWith(`${prefix}/`))) return home;
    parsed.searchParams.delete("return_to");
    return parsed.pathname + parsed.search + parsed.hash;
  } catch { return home; }
}

function currentAuthReturnPath(): string {
  if (typeof window === "undefined") return `${authBasePath()}/`;
  const { pathname, search, hash } = window.location;
  const carried = new URLSearchParams(search).get("return_to");
  return safeAuthReturnPath(carried || pathname + search + hash);
}

function withAuthReturnPath(entry: string): string {
  const target = currentAuthReturnPath();
  if (target === `${authBasePath()}/`) return entry;
  const url = new URL(entry, "https://return.invalid");
  url.searchParams.set("return_to", target);
  return url.origin === "https://return.invalid" ? url.pathname + url.search + url.hash : url.href;
}

export function consumeAuthReturnPath(): boolean {
  const url = new URL(window.location.href);
  const carried = url.searchParams.get("return_to");
  if (!carried) return false;
  const target = safeAuthReturnPath(carried);
  url.searchParams.delete("return_to");
  window.history.replaceState(window.history.state, "", url.pathname + url.search + url.hash);
  if (target === url.pathname + url.search + url.hash) return false;
  window.location.replace(target);
  return true;
}

export function authCompanyEntryPath(hostname = currentHostname()) {
  if (isCompanySSODomain(hostname)) {
    return withAuthReturnPath(`${authBasePath()}/`);
  }
  return "";
}

export function authLogoutPath(hostname = currentHostname()) {
  if (isCompanySSODomain(hostname)) {
    const normalizedHostname = normalizeCompanyHostname(hostname);
    const origin = companySSOOrigin(normalizedHostname);
    const gatewayOrigin = companySSOGatewayOrigin(normalizedHostname);
    const gatewayParams = new URLSearchParams({
      from_host: normalizedHostname,
      from_uri: `${origin}${currentAuthReturnPath()}`,
    });
    const gatewayLogout = `${gatewayOrigin}/api/g/sso/logouttogether?${gatewayParams.toString()}`;
    const localParams = new URLSearchParams({ redirect: gatewayLogout });
    return `${authBasePath()}/logout?${localParams.toString()}`;
  }
  return `${authBasePath()}/logout`;
}

export function shouldSkipLocalLogoutBeforeRedirect(hostname = currentHostname()) {
  return isCompanySSODomain(hostname);
}

export function shouldShowLogoutEntry(auth: Pick<AuthState, "enabled" | "authenticated"> | null, hostname = window.location.hostname) {
  if (!auth?.authenticated) return false;
  if (auth.enabled) return true;
  return isCompanySSODomain(hostname);
}

export function authUserDisplayName(user?: Pick<AuthUser, "display_name" | "username">) {
  return user?.display_name || user?.username || "已登录";
}

export function hasPermission(auth: Pick<AuthState, "permissions" | "user"> | null | undefined, permission: string) {
  return Boolean(auth?.permissions?.includes(permission) || auth?.user?.role === permission);
}

export function canManageUsers(auth: Pick<AuthState, "permissions" | "user"> | null | undefined) {
  return hasPermission(auth, "user:manage") || auth?.user?.role === "admin";
}

export function canEditStores(auth: Pick<AuthState, "permissions" | "user"> | null | undefined) {
  return hasPermission(auth, "store:write") || auth?.user?.role === "admin" || auth?.user?.role === "editor";
}

function authBasePath() {
  const base = import.meta.env.BASE_URL || "/erzhuang-project/";
  const normalized = base.startsWith("/") ? base : `/${base}`;
  const firstSegment = normalized.split("/").filter(Boolean)[0];
  return firstSegment ? `/${firstSegment}` : "/erzhuang-project";
}

function currentHostname() {
  return typeof window === "undefined" ? "" : window.location.hostname;
}

function isCompanySSODomain(hostname: string) {
  const normalized = normalizeCompanyHostname(hostname);
  return normalized === "lite.sy.soyoung.com" || normalized === "lite.soyoung.com";
}

function companySSOOrigin(hostname: string) {
  const normalized = normalizeCompanyHostname(hostname);
  if (normalized === "lite.sy.soyoung.com") return "https://lite.sy.soyoung.com";
  if (normalized === "lite.soyoung.com") return "http://lite.soyoung.com";
  return "";
}

function companySSOGatewayOrigin(hostname: string) {
  const normalized = normalizeCompanyHostname(hostname);
  if (normalized === "lite.sy.soyoung.com") return "https://security-test.sy.soyoung.com";
  if (normalized === "lite.soyoung.com") return "https://security.soyoung.com";
  return "";
}

function normalizeCompanyHostname(hostname: string) {
  return hostname.trim().toLowerCase().replace(/\.$/, "");
}
