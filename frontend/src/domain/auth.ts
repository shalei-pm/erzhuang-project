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
  login_url?: string;
  user?: AuthUser;
  permissions?: string[];
};

export function shouldShowLoginWelcome(auth: Pick<AuthState, "enabled" | "authenticated"> | null) {
  return Boolean(auth?.enabled && !auth.authenticated);
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
    return "/logout";
  }
  return `${authBasePath()}/logout`;
}

export function shouldUseGatewayLogout(hostname = currentHostname()) {
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
