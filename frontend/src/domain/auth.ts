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

export function authLogoutPath() {
  return `${authBasePath()}/logout`;
}

function authBasePath() {
  const base = import.meta.env.BASE_URL || "/erzhuang-project/";
  const normalized = base.startsWith("/") ? base : `/${base}`;
  const firstSegment = normalized.split("/").filter(Boolean)[0];
  return firstSegment ? `/${firstSegment}` : "/erzhuang-project";
}
