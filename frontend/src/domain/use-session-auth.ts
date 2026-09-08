import { useCallback, useEffect, useRef, useState } from "react";
import { storeSpaceApi } from "../api";
import { watchSessionDeadline } from "./session-deadline";
import {
  authCompanyEntryPath,
  authEntryPath,
  authStateFromError,
  claimIdleSessionTimeoutRedirect,
  claimSessionRedirect,
  consumeAuthReturnPath,
  idleSessionTimeoutRedirectKey,
  isIdleSessionTimeout,
  isSessionLoginRequired,
  removeSessionStorage,
  safeSessionStorage,
  sessionLoginRedirectKey,
  shouldShowLoginWelcome,
  subscribeSessionAuthRequired,
  type AuthState,
} from "./auth";

export function useSessionAuth(entryRedirectKey: string) {
  const [auth, setAuth] = useState<AuthState | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const invalidatedRef = useRef(false);
  const companyEntryRedirectAttemptedRef = useRef(false);

  const handleAuthRequired = useCallback((error?: unknown) => {
    if (invalidatedRef.current) return;
    invalidatedRef.current = true;
    const nextAuth = authStateFromError(error ?? { status: 401 });
    setAuth(nextAuth);
    setAuthLoading(false);
    if (isIdleSessionTimeout(error)) {
      if (claimIdleSessionTimeoutRedirect(safeSessionStorage())) {
        window.location.assign(authEntryPath(nextAuth));
      }
    } else if (isSessionLoginRequired(error)) {
      if (claimSessionRedirect(sessionLoginRedirectKey, safeSessionStorage(), true)) {
        window.location.assign(authEntryPath(nextAuth));
      }
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    const unsubscribe = subscribeSessionAuthRequired(handleAuthRequired);
    void storeSpaceApi.getAuthMe().then((nextAuth) => {
      // A late auth check must not revive a session already rejected by another API.
      if (cancelled || invalidatedRef.current) return;
      if (!nextAuth || typeof nextAuth.authenticated !== "boolean") {
        throw new Error("Invalid auth response");
      }
      if (!nextAuth.authenticated && nextAuth.code && !nextAuth.forbidden) {
        handleAuthRequired({ ...nextAuth, status: 401 });
        return;
      }
      if (nextAuth.authenticated && !nextAuth.forbidden && consumeAuthReturnPath()) return;
      setAuth(nextAuth);
      if (nextAuth.authenticated) {
        removeSessionStorage("erzhuang:sso-entry-redirected");
        removeSessionStorage("erzhuang:h5-sso-entry-redirected");
        removeSessionStorage(idleSessionTimeoutRedirectKey);
        removeSessionStorage(sessionLoginRedirectKey);
      }
    }).catch((error) => {
      if (!cancelled) handleAuthRequired(error);
    }).finally(() => {
      if (!cancelled) setAuthLoading(false);
    });
    return () => { cancelled = true; unsubscribe(); };
  }, [handleAuthRequired]);

  useEffect(() => {
    if (!auth?.authenticated || auth.forbidden || !auth.session) return;
    return watchSessionDeadline(auth.session, () => storeSpaceApi.getSessionStatus(), handleAuthRequired);
  }, [auth, handleAuthRequired]);

  useEffect(() => {
    if (!shouldShowLoginWelcome(auth)) return;
    // Structured session failures already chose logout/callback; never replace them with home.
    if (auth?.code === "auth_check_failed" || auth?.code === "session_login_required" || isIdleSessionTimeout({ status: 401, code: auth?.code })) return;
    const companyEntryPath = authCompanyEntryPath();
    if (!companyEntryPath || companyEntryRedirectAttemptedRef.current) return;
    companyEntryRedirectAttemptedRef.current = true;
    if (claimSessionRedirect(entryRedirectKey, safeSessionStorage())) window.location.replace(companyEntryPath);
  }, [auth, entryRedirectKey]);

  return { auth, setAuth, authLoading, handleAuthRequired };
}
