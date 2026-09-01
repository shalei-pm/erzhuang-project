package app

import (
	"context"
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newIdleSessionTestHandler(t *testing.T, sessions authSessionStore, now time.Time) (*Handler, *rsa.PrivateKey) {
	t.Helper()
	store := NewMemoryStore()
	if err := store.setAuthUserForTest(AuthUserRecord{
		ID: 42, Email: "idle@example.com", Username: "idle-user", DisplayName: "Idle User",
		Role: RoleAdmin, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	privateKey := newTestRSAKey(t)
	h := &Handler{
		store:            store,
		authSessionStore: sessions,
		auditRecorder:    store,
		auth: AuthConfig{
			Enabled:      true,
			CookieName:   "sy_sso_token",
			JWTPublicKey: publicKeyPEM(t, &privateKey.PublicKey),
			ClockSkew:    30 * time.Second,
			RequireEmail: true,
		},
		now:         func() time.Time { return now },
		idleTimeout: defaultAuthIdleTimeout,
	}
	return h, privateKey
}

func idleSessionRequest(t *testing.T, privateKey *rsa.PrivateKey, path string, now time.Time) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.AddCookie(&http.Cookie{Name: "sy_sso_token", Value: signAPISIXSSOToken(t, privateKey, map[string]any{
		"data": map[string]string{"mail": "idle@example.com", "username": "idle-user", "display": "Idle User"},
		"exp":  now.Add(time.Hour).Unix(),
	})})
	return request
}

func TestIdleAuthGateCreatesHttpOnlyLocalSession(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	sessions := newFakeAuthSessionStore()
	h, privateKey := newIdleSessionTestHandler(t, sessions, now)
	called := false
	recorder := httptest.NewRecorder()
	h.authGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, idleSessionRequest(t, privateKey, "/api/protected", now))

	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("gate result = status %d called=%t", recorder.Code, called)
	}
	var localCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == authSessionCookieName {
			localCookie = cookie
		}
	}
	if localCookie == nil || !localCookie.HttpOnly || localCookie.Value == "" {
		t.Fatalf("local session cookie = %s", summarizeCookie(localCookie))
	}
}

func TestAuthMeUsesIdleSessionGate(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	sessions := newFakeAuthSessionStore()
	h, privateKey := newIdleSessionTestHandler(t, sessions, now)
	recorder := httptest.NewRecorder()
	h.authGate(http.HandlerFunc(h.authMeHandler)).ServeHTTP(recorder, idleSessionRequest(t, privateKey, "/api/auth/me", now))

	if recorder.Code != http.StatusOK {
		t.Fatalf("auth/me status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response AuthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Authenticated || response.User == nil || response.User.Email != "idle@example.com" {
		t.Fatalf("auth/me response = %#v", response)
	}
	if !hasCookie(recorder.Result().Cookies(), authSessionCookieName) {
		t.Fatalf("auth/me did not issue local session cookie: %s", summarizeCookies(recorder.Result().Cookies()))
	}
}

func TestIdleAuthGateTouchesActiveSession(t *testing.T) {
	createdAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	sessions := newFakeAuthSessionStore()
	h, privateKey := newIdleSessionTestHandler(t, sessions, createdAt)
	first := httptest.NewRecorder()
	h.authGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(first, idleSessionRequest(t, privateKey, "/api/protected", createdAt))
	var localCookie *http.Cookie
	for _, cookie := range first.Result().Cookies() {
		if cookie.Name == authSessionCookieName {
			localCookie = cookie
		}
	}
	if localCookie == nil {
		t.Fatal("first request did not create local session cookie")
	}

	touchAt := createdAt.Add(5 * time.Minute)
	h.now = func() time.Time { return touchAt }
	secondRequest := idleSessionRequest(t, privateKey, "/api/protected", touchAt)
	secondRequest.AddCookie(localCookie)
	second := httptest.NewRecorder()
	h.authGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(second, secondRequest)

	if second.Code != http.StatusNoContent {
		t.Fatalf("active request status = %d body=%s", second.Code, second.Body.String())
	}
	hash := hashAuthSessionToken(localCookie.Value)
	sessions.mu.Lock()
	got := sessions.sessions[sessionHashKey(hash)]
	sessions.mu.Unlock()
	if !got.lastActivity.Equal(touchAt) {
		t.Fatalf("last activity = %s, want %s", got.lastActivity, touchAt)
	}
}

func TestIdleAuthGateExpiresSessionAndAuditsWithoutLeakingToken(t *testing.T) {
	createdAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	sessions := newFakeAuthSessionStore()
	h, privateKey := newIdleSessionTestHandler(t, sessions, createdAt)
	first := httptest.NewRecorder()
	h.authGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(first, idleSessionRequest(t, privateKey, "/api/protected", createdAt))
	var localCookie *http.Cookie
	for _, cookie := range first.Result().Cookies() {
		if cookie.Name == authSessionCookieName {
			localCookie = cookie
		}
	}
	if localCookie == nil {
		t.Fatal("first request did not create local session cookie")
	}

	expiredAt := createdAt.Add(defaultAuthIdleTimeout + time.Second)
	h.now = func() time.Time { return expiredAt }
	request := idleSessionRequest(t, privateKey, "/api/protected", expiredAt)
	request.AddCookie(localCookie)
	recorder := httptest.NewRecorder()
	h.authGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expired session reached protected handler")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expired request status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response AuthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != "session_idle_timeout" {
		t.Fatalf("auth response code = %q", response.Code)
	}
	hash := hashAuthSessionToken(localCookie.Value)
	sessions.mu.Lock()
	got := sessions.sessions[sessionHashKey(hash)]
	sessions.mu.Unlock()
	if got.revokeReason != "idle_timeout" || got.revokedAt.IsZero() {
		t.Fatalf("session revoke = reason %q at %s", got.revokeReason, got.revokedAt)
	}
	logs, err := h.store.ListAuditLogs(context.Background(), AuditLogFilter{
		StartAt: time.Unix(0, 0), EndAt: expiredAt.Add(time.Hour), PageSize: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs.Items) != 1 || logs.Items[0].Action != "auth.idle_timeout" {
		t.Fatalf("idle audit logs = %#v", logs.Items)
	}
	if !hasClearedAuthCookie(recorder.Result().Cookies(), authSessionCookieName, "") ||
		!hasClearedAuthCookie(recorder.Result().Cookies(), "sy_sso_token", "") {
		t.Fatalf("timeout did not clear both auth cookies: %s", summarizeCookies(recorder.Result().Cookies()))
	}
	if strings.Contains(recorder.Body.String(), localCookie.Value) {
		t.Fatal("timeout response leaked local token")
	}
}

func TestIdleAuthGateFailsClosedWithoutSessionStore(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	h, privateKey := newIdleSessionTestHandler(t, nil, now)
	recorder := httptest.NewRecorder()
	h.authGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request without session store reached protected handler")
	})).ServeHTTP(recorder, idleSessionRequest(t, privateKey, "/api/protected", now))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing session store status = %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRequirePermissionUsesIdleTimeoutResponse(t *testing.T) {
	createdAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	sessions := newFakeAuthSessionStore()
	h, privateKey := newIdleSessionTestHandler(t, sessions, createdAt)
	first := httptest.NewRecorder()
	h.authGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(first, idleSessionRequest(t, privateKey, "/api/protected", createdAt))
	var localCookie *http.Cookie
	for _, cookie := range first.Result().Cookies() {
		if cookie.Name == authSessionCookieName {
			localCookie = cookie
		}
	}
	if localCookie == nil {
		t.Fatal("first request did not create local session cookie")
	}

	expiredAt := createdAt.Add(defaultAuthIdleTimeout + time.Second)
	h.now = func() time.Time { return expiredAt }
	request := idleSessionRequest(t, privateKey, "/api/protected", expiredAt)
	request.AddCookie(localCookie)
	recorder := httptest.NewRecorder()
	if _, ok := h.requirePermission(recorder, request, PermissionStoreRead); ok {
		t.Fatal("expired session passed permission guard")
	}
	var response AuthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnauthorized || response.Code != "session_idle_timeout" {
		t.Fatalf("permission timeout response = status %d code %q", recorder.Code, response.Code)
	}
}

func TestIdleAuthTimeoutCannotExceedProductionDefault(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	h, _ := newIdleSessionTestHandler(t, newFakeAuthSessionStore(), now)
	h.idleTimeout = 4 * time.Hour
	if got := h.authIdleTimeout(); got != defaultAuthIdleTimeout {
		t.Fatalf("idle timeout = %s, want capped default %s", got, defaultAuthIdleTimeout)
	}
}

func TestIdleAuthGateExemptsHealthCallbackAndLogout(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	h, privateKey := newIdleSessionTestHandler(t, nil, now)
	for _, path := range []string{"/health", "/_/auth/callback", "/api/auth/logout", "/logout"} {
		t.Run(path, func(t *testing.T) {
			called := false
			recorder := httptest.NewRecorder()
			h.authGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(recorder, idleSessionRequest(t, privateKey, path, now))
			if recorder.Code != http.StatusNoContent || !called {
				t.Fatalf("exempt path result = status %d called=%t", recorder.Code, called)
			}
		})
	}
}

func hasCookie(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value != "" {
			return true
		}
	}
	return false
}

func summarizeCookie(cookie *http.Cookie) string {
	if cookie == nil {
		return "cookie_present=false http_only=false secure=false count=0 name=none"
	}
	return fmt.Sprintf("cookie_present=true http_only=%t secure=%t count=1 name=%q", cookie.HttpOnly, cookie.Secure, cookie.Name)
}

func summarizeCookies(cookies []*http.Cookie) string {
	names := make([]string, 0, len(cookies))
	allHTTPOnly := len(cookies) > 0
	anySecure := false
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		names = append(names, cookie.Name)
		allHTTPOnly = allHTTPOnly && cookie.HttpOnly
		anySecure = anySecure || cookie.Secure
	}
	return fmt.Sprintf("cookie_present=%t http_only=%t secure=%t count=%d names=%q", len(names) > 0, allHTTPOnly, anySecure, len(names), strings.Join(names, ","))
}

func TestManualLogoutRevokesLocalSessionAndClearsCookies(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	sessions := newFakeAuthSessionStore()
	h, privateKey := newIdleSessionTestHandler(t, sessions, now)
	token, err := sessions.CreateAuthSession(context.Background(), AuthSessionCreate{UserID: 42, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	request := idleSessionRequest(t, privateKey, "/api/auth/logout", now)
	request.AddCookie(&http.Cookie{Name: authSessionCookieName, Value: token})
	recorder := httptest.NewRecorder()
	h.authLogoutHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("logout status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	hash := hashAuthSessionToken(token)
	sessions.mu.Lock()
	got := sessions.sessions[sessionHashKey(hash)]
	sessions.mu.Unlock()
	if got.revokeReason != "manual_logout" || got.revokedAt.IsZero() {
		t.Fatalf("logout revoke = reason %q at %s", got.revokeReason, got.revokedAt)
	}
	if !hasClearedAuthCookie(recorder.Result().Cookies(), authSessionCookieName, "") ||
		!hasClearedAuthCookie(recorder.Result().Cookies(), "sy_sso_token", "") {
		t.Fatalf("logout did not clear both auth cookies: %s", summarizeCookies(recorder.Result().Cookies()))
	}
}

func sessionHashKey(hash [32]byte) string {
	return hex.EncodeToString(hash[:])
}
