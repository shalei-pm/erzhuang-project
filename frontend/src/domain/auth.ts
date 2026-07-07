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

export type AuthState = {
  enabled: boolean;
  authenticated: boolean;
  forbidden?: boolean;
  login_url?: string;
  user?: AuthUser;
  permissions?: string[];
};

export function shouldShowLoginWelcome(auth: Pick<AuthState, "enabled" | "authenticated" | "forbidden"> | null) {
  return Boolean(auth?.enabled && !auth.authenticated && !auth.forbidden);
}

export function shouldShowForbiddenAccess(auth: Pick<AuthState, "forbidden"> | null) {
  return Boolean(auth?.forbidden);
}

export function shouldBlockBusinessData(auth: Pick<AuthState, "enabled" | "authenticated" | "forbidden"> | null) {
  return shouldShowLoginWelcome(auth) || shouldShowForbiddenAccess(auth);
}

export function authLoginPath(loginUrl?: string) {
  if (loginUrl) return loginUrl;
  return `${authBasePath()}/_/auth/callback`;
}

export function authCompanyEntryPath(hostname = currentHostname()) {
  if (isCompanySSODomain(hostname)) {
    return `${authBasePath()}/`;
  }
  return "";
}

export function authLogoutPath(hostname = currentHostname()) {
  if (isCompanySSODomain(hostname)) {
    const params = new URLSearchParams({
      from_host: hostname,
      from_uri: `https://${hostname}${authBasePath()}/`,
    });
    return `https://security-test.sy.soyoung.com/api/g/sso/logouttogether?${params.toString()}`;
  }
  return `${authBasePath()}/logout`;
}

export function shouldSkipLocalLogoutBeforeRedirect(_hostname = currentHostname()) {
  return false;
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
  return hostname === "lite.sy.soyoung.com";
}
